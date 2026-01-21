#!/usr/bin/env python3

import argparse
import json
import os
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple

try:
    import requests
except ImportError:  # pragma: no cover
    print("Error: missing python dependency 'requests' (pip install requests)", file=sys.stderr)
    sys.exit(2)


DB_PARAMS = [
    "CHART_VERSION",
    "DATABASE_SIZE",
    "DATABASE_VERSION",
    "RESOURCES_REQUESTS_CPU",
    "RESOURCES_REQUESTS_MEMORY",
]

CORE_PARAMS = [
    "ALLOW_SUSPEND",
    "IMAGE_PULL_POLICY",
    "IMAGE_REPOSITORY",
    "IMAGE_TAG",
    "RESOURCES_REQUESTS_CPU",
    "RESOURCES_REQUESTS_MEMORY",
]


@dataclass(frozen=True)
class BackupItem:
    deployment_name: str
    saved_database_id: int
    deployment_id: Optional[int] = None


def _env_default(name: str) -> Optional[str]:
    v = os.environ.get(name)
    return v if v else None


class Auth:
    def __init__(self, session: requests.Session, host: str, token: Optional[str], user_email: Optional[str], password: Optional[str]):
        self._session = session
        self._host = host.rstrip("/")
        self._token = token
        self._user_email = user_email
        self._password = password

    def ensure_token(self) -> str:
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


def _request(session: requests.Session, auth: Auth, method: str, url: str, json_body: Optional[dict] = None) -> Any:
    headers = auth.auth_header()
    resp = session.request(method, url, headers=headers, json=json_body, timeout=60)
    if resp.status_code == 401:
        auth._token = None
        headers = auth.auth_header()
        resp = session.request(method, url, headers=headers, json=json_body, timeout=60)
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


def _find_deployment_by_name(group_deployments: List[dict], name: str) -> Optional[dict]:
    for d in group_deployments:
        if d.get("name") == name:
            return d
    return None


def _find_instance(deployment: dict, stack_name: str) -> Optional[dict]:
    for inst in deployment.get("instances") or []:
        if inst.get("stackName") == stack_name:
            return inst
    return None


def _pick_params(instance_details: dict, keys: List[str]) -> Dict[str, str]:
    params = instance_details.get("parameters") or {}
    out: Dict[str, str] = {}
    for k in keys:
        v = ((params.get(k) or {}).get("value"))
        if not v or v == "***":
            continue
        out[k] = str(v)
    return out


def _load_backup_items(path: str) -> Tuple[str, List[BackupItem]]:
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    group = str(data.get("group") or "")
    items_raw = data.get("items") or []
    items: List[BackupItem] = []
    for it in items_raw:
        name = str(it.get("deploymentName"))
        db_id = int(it.get("savedDatabaseId"))
        dep_id = it.get("deploymentId")
        items.append(BackupItem(deployment_name=name, saved_database_id=db_id, deployment_id=int(dep_id) if dep_id is not None else None))
    return group, items


def _load_pgadmin_creds(path: str) -> Dict[str, Dict[str, str]]:
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)

    if isinstance(data, dict) and "items" in data and isinstance(data["items"], list):
        m: Dict[str, Dict[str, str]] = {}
        for it in data["items"]:
            dep = str(it.get("deploymentName") or it.get("name") or "")
            if not dep:
                continue
            m[dep] = {
                "PGADMIN_USERNAME": str(it.get("PGADMIN_USERNAME") or ""),
                "PGADMIN_PASSWORD": str(it.get("PGADMIN_PASSWORD") or ""),
            }
        return m

    if isinstance(data, dict):
        m2: Dict[str, Dict[str, str]] = {}
        for dep, vals in data.items():
            if not isinstance(vals, dict):
                continue
            m2[str(dep)] = {
                "PGADMIN_USERNAME": str(vals.get("PGADMIN_USERNAME") or ""),
                "PGADMIN_PASSWORD": str(vals.get("PGADMIN_PASSWORD") or ""),
            }
        return m2

    raise ValueError("Unsupported pgadmin creds JSON format")


def _to_api_params(values: Dict[str, str]) -> Dict[str, Dict[str, str]]:
    return {k: {"value": v} for k, v in values.items()}


def main() -> int:
    parser = argparse.ArgumentParser(description="Restore deployments from a backup JSON by deleting/recreating deployments and instances with selected parameters.")
    parser.add_argument("--group", required=True, help="Target group (same group as source)")
    parser.add_argument("--backup-json", required=True)
    parser.add_argument("--pgadmin-creds-json", required=True)
    parser.add_argument("--host", default=_env_default("IM_HOST"), help="IM API base URL (or set IM_HOST)")
    parser.add_argument("--token", default=_env_default("ACCESS_TOKEN"), help="Optional bearer token (or set ACCESS_TOKEN). If omitted, logs in using USER_EMAIL/PASSWORD.")
    parser.add_argument("--user-email", default=_env_default("USER_EMAIL"), help="Login email (or set USER_EMAIL)")
    parser.add_argument("--password", default=_env_default("PASSWORD"), help="Login password (or set PASSWORD)")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--force-delete", action="store_true")
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--deployment-name", default="", help="Only restore a single deployment by name")
    parser.add_argument("--out", default="", help="Optional JSON report output")

    args = parser.parse_args()

    if not args.host:
        print("Error: IM host missing. Provide --host or set IM_HOST.", file=sys.stderr)
        return 2

    host = args.host.rstrip("/")
    target_group = args.group

    backup_group, items = _load_backup_items(args.backup_json)
    if backup_group and backup_group != target_group:
        print(f"Warning: backup JSON group={backup_group} but restoring into group={target_group}", file=sys.stderr)

    pgadmin_creds = _load_pgadmin_creds(args.pgadmin_creds_json)

    if args.deployment_name:
        items = [i for i in items if i.deployment_name == args.deployment_name]

    if args.limit and args.limit > 0:
        items = items[: args.limit]

    if not items:
        print("No items to restore.", file=sys.stderr)
        return 1

    session = requests.Session()
    auth = Auth(session, host, args.token, args.user_email, args.password)

    deployments_json = _request(session, auth, "GET", f"{host}/deployments")
    group_deployments = _get_group_deployments(deployments_json, target_group)

    report: Dict[str, Any] = {
        "group": target_group,
        "createdAt": datetime.now(timezone.utc).isoformat(),
        "items": [],
    }

    for item in items:
        name = item.deployment_name
        saved_db_id = item.saved_database_id

        current = _find_deployment_by_name(group_deployments, name)
        if not current:
            raise RuntimeError(f"Deployment '{name}' not found in group '{target_group}'")

        current_id = int(current["id"])
        current_description = str(current.get("description") or "")
        current_ttl = int(current.get("ttl") or 0)

        db_inst = _find_instance(current, "dhis2-db")
        core_inst = _find_instance(current, "dhis2-core")
        pg_inst = _find_instance(current, "pgadmin")

        if not db_inst:
            print(f"Skipping {name}: no dhis2-db instance", file=sys.stderr)
            continue

        db_inst_id = int(db_inst["id"])
        db_details = _request(session, auth, "GET", f"{host}/instances/{db_inst_id}/details")
        db_values = _pick_params(db_details, DB_PARAMS)
        db_values["DATABASE_ID"] = str(saved_db_id)

        core_payload: Optional[dict] = None
        if core_inst:
            core_inst_id = int(core_inst["id"])
            core_details = _request(session, auth, "GET", f"{host}/instances/{core_inst_id}/details")
            core_values = _pick_params(core_details, CORE_PARAMS)
            core_values["DATABASE_ID"] = str(saved_db_id)
            core_public = bool(core_details.get("public"))
            core_payload = {
                "stackName": "dhis2-core",
                "public": core_public,
                "parameters": _to_api_params(core_values),
            }

        pg_payload: Optional[dict] = None
        if pg_inst:
            creds = pgadmin_creds.get(name) or {}
            user = str(creds.get("PGADMIN_USERNAME") or "")
            pwd = str(creds.get("PGADMIN_PASSWORD") or "")
            if not user or not pwd:
                raise RuntimeError(f"Missing pgadmin creds for deployment '{name}' in {args.pgadmin_creds_json}")
            pg_payload = {
                "stackName": "pgadmin",
                "parameters": _to_api_params({"PGADMIN_USERNAME": user, "PGADMIN_PASSWORD": pwd}),
            }

        print(f"[{name}] will recreate deployment: ttl={current_ttl} description_len={len(current_description)} savedDbId={saved_db_id}")
        if args.dry_run:
            print(f"[{name}] DRY-RUN: would DELETE /deployments/{current_id}")
            print(f"[{name}] DRY-RUN: would POST /deployments name={name} group={target_group}")
            print(f"[{name}] DRY-RUN: would POST instance dhis2-db params={list(db_values.keys())}")
            if core_payload:
                print(f"[{name}] DRY-RUN: would POST instance dhis2-core params={list(core_payload['parameters'].keys())} public={core_payload.get('public')}")
            if pg_payload:
                print(f"[{name}] DRY-RUN: would POST instance pgadmin params={list(pg_payload['parameters'].keys())}")
            print(f"[{name}] DRY-RUN: would POST /deployments/<newId>/deploy")
            report["items"].append(
                {
                    "deploymentName": name,
                    "sourceDeploymentId": current_id,
                    "newDeploymentId": None,
                    "savedDatabaseId": saved_db_id,
                    "dryRun": True,
                }
            )
            continue

        if not args.force_delete:
            raise RuntimeError(f"Refusing to delete/recreate '{name}' without --force-delete")

        _request(session, auth, "DELETE", f"{host}/deployments/{current_id}")

        new_dep = _request(
            session,
            auth,
            "POST",
            f"{host}/deployments",
            json_body={"name": name, "group": target_group, "description": current_description, "ttl": current_ttl},
        )
        new_id = int((new_dep or {}).get("id"))

        _request(
            session,
            auth,
            "POST",
            f"{host}/deployments/{new_id}/instance",
            json_body={"stackName": "dhis2-db", "parameters": _to_api_params(db_values)},
        )

        if core_payload:
            _request(session, auth, "POST", f"{host}/deployments/{new_id}/instance", json_body=core_payload)

        if pg_payload:
            _request(session, auth, "POST", f"{host}/deployments/{new_id}/instance", json_body=pg_payload)

        _request(session, auth, "POST", f"{host}/deployments/{new_id}/deploy")

        report["items"].append(
            {
                "deploymentName": name,
                "sourceDeploymentId": current_id,
                "newDeploymentId": new_id,
                "savedDatabaseId": saved_db_id,
                "dryRun": False,
            }
        )

        deployments_json = _request(session, auth, "GET", f"{host}/deployments")
        group_deployments = _get_group_deployments(deployments_json, target_group)

    if args.out:
        os.makedirs(os.path.dirname(os.path.abspath(args.out)) or ".", exist_ok=True)
        with open(args.out, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2, sort_keys=True)
            f.write("\n")
        print(f"Wrote report to {args.out}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
