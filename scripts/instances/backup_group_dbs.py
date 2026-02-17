#!/usr/bin/env python3

import argparse
import json
import os
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from datetime import datetime, timezone
from threading import Lock
from typing import Any, Dict, List, Optional, Union

try:
    import requests
except ImportError:  # pragma: no cover
    print("Error: missing python dependency 'requests' (pip install requests)", file=sys.stderr)
    sys.exit(2)


@dataclass(frozen=True)
class BackupItem:
    deployment_id: int
    deployment_name: str
    db_instance_id: int
    saved_database_id: int
    saved_database_name: str
    original_database_id: Union[int, str]  # numeric id or slug (e.g. "whoami-2-42-sql-gz")


@dataclass
class PendingBackup:
    deployment_id: int
    deployment_name: str
    db_instance_id: int
    saved_database_id: int
    saved_database_name: str
    original_database_id: Union[int, str]


def _env_default(name: str) -> Optional[str]:
    value = os.environ.get(name)
    return value if value else None


class Auth:
    def __init__(self, session: requests.Session, host: str, token: Optional[str], user_email: Optional[str], password: Optional[str]):
        self._session = session
        self._host = host.rstrip("/")
        self._token = token
        self._user_email = user_email
        self._password = password
        self._lock = Lock()

    def ensure_token(self) -> str:
        with self._lock:
            if self._token:
                return self._token
            if not self._user_email or not self._password:
                raise RuntimeError("Missing credentials: set USER_EMAIL and PASSWORD (or pass --user-email/--password).")
            resp = self._session.post(
                f"{self._host}/tokens",
                auth=(self._user_email, self._password),
                json={},
                timeout=60,
            )
            if not resp.ok:
                raise RuntimeError(f"HTTP {resp.status_code} POST {self._host}/tokens: {resp.text}")
            token = resp.cookies.get("accessToken")
            if token:
                self._token = token
                return token
            try:
                data = resp.json()
            except Exception:
                data = {}
            token2 = data.get("accessToken") if isinstance(data, dict) else None
            if token2:
                self._token = str(token2)
                return self._token
            raise RuntimeError("Login succeeded but could not find access token in cookies or JSON response.")

    def auth_header(self) -> Dict[str, str]:
        return {"Authorization": f"Bearer {self.ensure_token()}"}

    def clear_token(self) -> None:
        with self._lock:
            self._token = None


def _request(session: requests.Session, auth: Auth, method: str, url: str, json_body: Optional[dict] = None, timeout: int = 60) -> Any:
    headers = auth.auth_header()
    resp = session.request(method, url, headers=headers, json=json_body, timeout=timeout)
    if resp.status_code == 401:
        auth.clear_token()
        headers = auth.auth_header()
        resp = session.request(method, url, headers=headers, json=json_body, timeout=timeout)
    if not resp.ok:
        raise RuntimeError(f"HTTP {resp.status_code} {method} {url}: {resp.text}")
    if resp.text.strip() == "":
        return None
    return resp.json()


def _get_group_deployments(deployments_json: list, group: str) -> List[dict]:
    for g in deployments_json:
        if g.get("name") == group:
            return g.get("deployments") or []
    return []


def _find_instance_id(deployment: dict, stack_name: str) -> Optional[int]:
    for inst in deployment.get("instances") or []:
        if inst.get("stackName") == stack_name:
            inst_id = inst.get("id")
            return int(inst_id) if inst_id is not None else None
    return None


def _fetch_instance_details(
    session: requests.Session, auth: Auth, host: str, instance_id: int
) -> Optional[dict]:
    """GET /instances/{id}/details returns one instance with parameters (same as frontend). DATABASE_ID is not sensitive so it is stored and returned in plaintext."""
    try:
        return _request(session, auth, "GET", f"{host}/instances/{instance_id}/details", timeout=60)
    except Exception as e:
        print(f"WARNING: failed to fetch instance {instance_id} details: {e}", file=sys.stderr)
        return None


def _get_original_database_id_from_instance(instance: dict) -> Optional[Union[int, str]]:
    """Return the DATABASE_ID parameter value from an instance details response (numeric id or slug), or None if missing."""
    params = instance.get("parameters") or {}
    db_id_param = params.get("DATABASE_ID")
    if not isinstance(db_id_param, dict):
        return None
    value = db_id_param.get("value")
    if value is None:
        return None
    if isinstance(value, (int, float)):
        return int(value)
    if isinstance(value, str) and value.strip():
        try:
            return int(value)
        except (TypeError, ValueError):
            return value  # slug, e.g. "whoami-2-42-sql-gz"
    return None


def _log_missing_database_id_debug(dep_name: str, dep_id: int, instance: dict) -> None:
    """Log what we got from the API so we can see why DATABASE_ID is missing or unparseable."""
    params = instance.get("parameters") or {}
    param_names = list(params.keys())[:15]
    if len(params) > 15:
        param_names.append("...")
    db_id_raw = params.get("DATABASE_ID")
    print(
        f"Skipping {dep_name} ({dep_id}): no DATABASE_ID parameter on dhis2-db instance. "
        f"Instance id={instance.get('id')}, parameters keys ({len(params)}): {param_names}. "
        f"DATABASE_ID present={db_id_raw is not None}, value={repr(db_id_raw)}",
        file=sys.stderr,
    )


def _wait_for_database_ready(
    session: requests.Session,
    host: str,
    auth: Auth,
    db_id: int,
    poll_interval: int,
    timeout_seconds: int,
) -> Dict[str, Any]:
    deadline = time.time() + timeout_seconds
    last: Optional[dict] = None
    while time.time() < deadline:
        info = _request(session, auth, "GET", f"{host}/databases/{db_id}")
        last = info if isinstance(info, dict) else None
        url = (last or {}).get("url")
        size = (last or {}).get("size")
        try:
            size_int = int(size) if size is not None else 0
        except Exception:
            size_int = 0
        if url and size_int > 0:
            return last or {}
        time.sleep(poll_interval)
    raise TimeoutError(f"Timed out waiting for database {db_id} to have url and size>0. Last: {json.dumps(last or {}, indent=2)}")


def _start_backup(
    session: requests.Session,
    host: str,
    auth: Auth,
    dep_id: int,
    dep_name: str,
    db_instance_id: int,
    backup_name: str,
    fmt: str,
    original_database_id: Union[int, str],
) -> PendingBackup:
    print(f"[{dep_name}] saving database via save-as on instance {db_instance_id} as '{backup_name}'")
    save_resp = _request(
        session,
        auth,
        "POST",
        f"{host}/databases/save-as/{db_instance_id}",
        json_body={"name": backup_name, "format": fmt},
        timeout=120,
    )
    saved_db_id_raw = (save_resp or {}).get("id")
    if saved_db_id_raw is None:
        raise RuntimeError(f"[{dep_name}] save-as response missing id: {save_resp}")
    saved_db_id = int(saved_db_id_raw)
    print(f"[{dep_name}] started backup, database id={saved_db_id}")
    return PendingBackup(
        deployment_id=dep_id,
        deployment_name=dep_name,
        db_instance_id=db_instance_id,
        saved_database_id=saved_db_id,
        saved_database_name=backup_name,
        original_database_id=original_database_id,
    )


def _wait_and_finalize(
    session: requests.Session,
    host: str,
    auth: Auth,
    pending: PendingBackup,
    poll_interval: int,
    timeout_seconds: int,
) -> BackupItem:
    print(f"[{pending.deployment_name}] waiting for url+size on database id={pending.saved_database_id}")
    info = _wait_for_database_ready(session, host, auth, pending.saved_database_id, poll_interval, timeout_seconds)
    print(f"[{pending.deployment_name}] ready: url={info.get('url')} size={info.get('size')}")
    return BackupItem(
        deployment_id=pending.deployment_id,
        deployment_name=pending.deployment_name,
        db_instance_id=pending.db_instance_id,
        saved_database_id=pending.saved_database_id,
        saved_database_name=pending.saved_database_name,
        original_database_id=pending.original_database_id,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Save-As all dhis2-db databases in a group and write a deployment->saved db mapping JSON.")
    parser.add_argument("--group", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--host", default=_env_default("IM_HOST"), help="IM API base URL (or set IM_HOST)")
    parser.add_argument("--token", default=_env_default("ACCESS_TOKEN"), help="Optional bearer token (or set ACCESS_TOKEN). If omitted, logs in using USER_EMAIL/PASSWORD.")
    parser.add_argument("--user-email", default=_env_default("USER_EMAIL"), help="Login email (or set USER_EMAIL)")
    parser.add_argument("--password", default=_env_default("PASSWORD"), help="Login password (or set PASSWORD)")
    parser.add_argument("--poll-interval", type=int, default=10)
    parser.add_argument("--timeout-seconds", type=int, default=600)
    parser.add_argument("--format", default="plain", choices=["plain", "custom"], help="Database backup format")
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--deployment-name", default="", help="Only back up a single deployment by name")
    parser.add_argument("--parallel", action="store_true", help="Start all backups first, then poll for completion concurrently")

    args = parser.parse_args()

    if not args.host:
        print("Error: IM host missing. Provide --host or set IM_HOST.", file=sys.stderr)
        return 2

    host = args.host.rstrip("/")
    group = args.group

    session = requests.Session()
    auth = Auth(session, host, args.token, args.user_email, args.password)

    deployments_json = _request(session, auth, "GET", f"{host}/deployments")
    group_deployments = _get_group_deployments(deployments_json, group)

    if args.deployment_name:
        group_deployments = [d for d in group_deployments if d.get("name") == args.deployment_name]

    if not group_deployments:
        print(f"No deployments found in group: {group}", file=sys.stderr)
        return 1

    if args.limit and args.limit > 0:
        group_deployments = group_deployments[: args.limit]

    targets: List[tuple] = []
    for dep in group_deployments:
        dep_id = int(dep["id"])
        dep_name = str(dep["name"])
        db_instance_id = _find_instance_id(dep, "dhis2-db")
        if db_instance_id is None:
            print(f"Skipping {dep_name} ({dep_id}): no dhis2-db instance", file=sys.stderr)
            continue
        # GET /deployments list does not include instance parameters. Fetch this instance's details
        # (GET /instances/{id}/details); DATABASE_ID is not sensitive so it is returned in plaintext.
        instance_details = _fetch_instance_details(session, auth, host, db_instance_id)
        if instance_details is None:
            print(f"Skipping {dep_name} ({dep_id}): failed to fetch instance details", file=sys.stderr)
            continue
        original_db_id = _get_original_database_id_from_instance(instance_details)
        if original_db_id is None:
            _log_missing_database_id_debug(dep_name, dep_id, instance_details)
            continue
        backup_name = f"{dep_name}-backup-{datetime.now(timezone.utc).strftime('%Y%m%d-%H%M%S')}.sql.gz"
        targets.append((dep_id, dep_name, db_instance_id, backup_name, original_db_id))

    if not targets:
        print("No valid deployments to back up.", file=sys.stderr)
        return 1

    items: List[BackupItem] = []
    errors: List[str] = []

    if args.parallel:
        pending: List[PendingBackup] = []
        for dep_id, dep_name, db_instance_id, backup_name, original_db_id in targets:
            try:
                p = _start_backup(
                    session, host, auth, dep_id, dep_name, db_instance_id, backup_name, args.format, original_db_id
                )
                pending.append(p)
            except Exception as e:
                errors.append(f"[{dep_name}] start failed: {e}")
                print(f"[{dep_name}] ERROR starting backup: {e}", file=sys.stderr)

        if pending:
            print(f"Started {len(pending)} backup(s), polling for completion concurrently...")
            with ThreadPoolExecutor(max_workers=len(pending)) as executor:
                wait_futures = {
                    executor.submit(
                        _wait_and_finalize, requests.Session(), host, auth, p, args.poll_interval, args.timeout_seconds
                    ): p.deployment_name
                    for p in pending
                }
                for future in as_completed(wait_futures):
                    dep_name = wait_futures[future]
                    try:
                        items.append(future.result())
                    except Exception as e:
                        errors.append(f"[{dep_name}] wait failed: {e}")
                        print(f"[{dep_name}] ERROR waiting for backup: {e}", file=sys.stderr)
    else:
        for dep_id, dep_name, db_instance_id, backup_name, original_db_id in targets:
            try:
                p = _start_backup(
                    session, host, auth, dep_id, dep_name, db_instance_id, backup_name, args.format, original_db_id
                )
                item = _wait_and_finalize(session, host, auth, p, args.poll_interval, args.timeout_seconds)
                items.append(item)
            except Exception as e:
                errors.append(f"[{dep_name}] failed: {e}")
                print(f"[{dep_name}] ERROR: {e}", file=sys.stderr)

    def item_to_dict(i: BackupItem) -> dict:
        d = {
            "deploymentId": i.deployment_id,
            "deploymentName": i.deployment_name,
            "dbInstanceId": i.db_instance_id,
            "savedDatabaseId": i.saved_database_id,
            "savedDatabaseName": i.saved_database_name,
        }
        d["originalDatabaseId"] = i.original_database_id  # int or str (slug)
        return d

    out_obj = {
        "group": group,
        "createdAt": datetime.now(timezone.utc).isoformat(),
        "items": [item_to_dict(i) for i in items],
    }

    os.makedirs(os.path.dirname(os.path.abspath(args.out)) or ".", exist_ok=True)
    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(out_obj, f, indent=2, sort_keys=True)
        f.write("\n")

    print(f"Wrote {len(items)} item(s) to {args.out}")
    if errors:
        print(f"Errors ({len(errors)}):", file=sys.stderr)
        for e in errors:
            print(f"  {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
