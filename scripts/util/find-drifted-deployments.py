#!/usr/bin/env python3

"""
Find Drifted Deployments

This script compares what Instance Manager believes it is running against what the
clusters actually run, in the direction the orphan checks do not cover. Those checks ask
whether the cluster holds something Instance Manager has forgotten. This one asks whether
Instance Manager is holding a record of something the cluster no longer runs.

Two checks, both of defects rather than of states. Neither condition is something a user
should learn to recognise and work around, so nothing here is surfaced in the product:
every finding is a bug report about Instance Manager or about the cluster it drives.

1. Deployed instances with no workloads. An instance whose deploy status is "deployed"
   has helmfile-installed workloads behind it, so no pod carrying its im-id label means
   something removed them without Instance Manager noticing: a destroy that half
   succeeded, a namespace rebuilt underneath us, or a helm uninstall run by hand.

2. Deployments outliving their TTL. The inspector destroys a deployment once its TTL
   expires, and a destroy that fails leaves the row in place (DeleteDeployment keeps the
   row when DestroyInstance errors). The TTL is still expired, so the inspector retries
   every two minutes, forever, logging an error nobody reads. A deployment still standing
   well past its TTL is that loop running.

Check 2 needs no cluster access, so it still reports when every kubeconfig is expired.

Clusters are named rather than guessed: --cluster NAME=PATH ties a kubeconfig to the
cluster of that name in Instance Manager. A cluster Instance Manager knows about that was
given no kubeconfig is reported as unchecked rather than silently skipped, and its
instances are left out of check 1 instead of being called drifted. An empty cache or an
unreadable cluster must never read as "nothing is running there", which would report
every instance on it at once.
"""

import argparse
import json
import os
import shlex
import subprocess
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import Dict, List, Optional, Set, Tuple

import requests


ENVIRONMENTS = [("prod", "IM_HOST_PROD"), ("dev", "IM_HOST_DEV")]
INSTANCE_ID_LABEL = "im-id"
DEPLOYED_STATUS = "deployed"
DEFAULT_TTL_GRACE_HOURS = 1


@dataclass
class Instance:
    id: int
    name: str
    stack: str
    deploy_status: str


@dataclass
class Deployment:
    id: int
    name: str
    group: str
    namespace: str
    cluster_id: Optional[int]
    environment: str
    created_at: Optional[datetime]
    ttl: int
    instances: List[Instance]

    @property
    def expires_at(self) -> Optional[datetime]:
        if self.created_at is None or self.ttl <= 0:
            return None
        return self.created_at + timedelta(seconds=self.ttl)


@dataclass
class ClusterReport:
    """What one cluster's pods say. A cluster that could not be read carries its error and
    contributes nothing to the findings, so an expired kubeconfig shows up as a gap in
    coverage rather than as every instance on that cluster having lost its workloads."""

    name: str
    kubeconfig: Optional[str] = None
    namespaces_by_instance_id: Dict[int, Set[str]] = field(default_factory=dict)
    error: Optional[str] = None

    @property
    def checked(self) -> bool:
        return self.kubeconfig is not None and self.error is None


@dataclass
class MissingWorkloads:
    deployment: Deployment
    instance: Instance


@dataclass
class ExpiredDeployment:
    deployment: Deployment
    overdue: timedelta


class ClusterUnreachable(Exception):
    """One cluster could not be read. The others are still worth reporting, so this is
    caught per cluster rather than ending the run."""


def summarise_stderr(stderr: str) -> str:
    lines = [line.strip() for line in stderr.splitlines() if line.strip()]
    for line in reversed(lines):
        if line.startswith("error:"):
            return line[len("error:"):].strip()
    return lines[-1] if lines else "no error output"


def print_separator(title: str = None):
    if title:
        print("\n" + "=" * 60)
        print(title)
    print("=" * 60)


def kubectl_json(kubeconfig: str, *args: str) -> dict:
    cmd = ["kubectl", "--kubeconfig", kubeconfig, *args, "--output", "json"]
    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode != 0:
        raise ClusterUnreachable(summarise_stderr(result.stderr))

    return json.loads(result.stdout)


def parse_timestamp(value: str) -> Optional[datetime]:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)
    except (TypeError, ValueError):
        return None


def authenticate(im_host: str, environment: str) -> str:
    user_type = f"User_{environment}"
    user_email = os.environ.get(f"USER_EMAIL_{environment.upper()}")
    password = os.environ.get(f"PASSWORD_{environment.upper()}")

    if not user_email or not password:
        print(f"Error: Credentials for {environment} environment not found. Set USER_EMAIL_{environment.upper()} and PASSWORD_{environment.upper()}")
        sys.exit(1)

    env = os.environ.copy()
    env["IM_HOST"] = im_host
    env["USER_EMAIL"] = user_email
    env["PASSWORD"] = password

    script_dir = os.path.dirname(os.path.abspath(__file__))
    auth_script = os.path.join(script_dir, "..", "clusters", "auth.sh")
    cmd = f"source {shlex.quote(auth_script)} {shlex.quote(user_type)} && echo $ACCESS_TOKEN"

    result = subprocess.run(["bash", "-c", cmd], env=env, capture_output=True, text=True, check=True)
    access_token = result.stdout.strip()

    if not access_token:
        print(f"Failed to obtain access token for {im_host}")
        if result.stderr:
            print(f"Error output: {result.stderr}")
        sys.exit(1)

    return access_token


def get_json(access_token: str, url: str):
    try:
        response = requests.get(url, headers={"Authorization": f"Bearer {access_token}"}, timeout=30)
        response.raise_for_status()
    except requests.exceptions.RequestException as e:
        print(f"Error fetching {url}: {e}")
        response = getattr(e, "response", None)
        if response is not None:
            print(f"Response status: {response.status_code}")
            print(f"Response body: {response.text}")
        sys.exit(1)

    if not response.text.strip():
        return []

    return response.json()


def get_cluster_names(access_token: str, im_host: str, environment: str) -> Dict[Tuple[str, int], str]:
    """Cluster names keyed by environment as well as id. Prod and dev are separate databases
    whose cluster ids collide, so merging them on id alone points a dev deployment at whichever
    cluster prod happens to have under the same number."""
    clusters = get_json(access_token, f"{im_host}/clusters")
    return {(environment, cluster["id"]): cluster["name"].strip() for cluster in clusters}


def get_deployments(access_token: str, im_host: str, environment: str) -> List[Deployment]:
    print(f"Fetching deployments from Instance Manager ({environment})...")
    groups = get_json(access_token, f"{im_host}/deployments")

    deployments = []
    for group_data in groups:
        for deployment_data in group_data.get("deployments") or []:
            group = deployment_data.get("group") or {}
            instances = [
                Instance(
                    id=instance_data["id"],
                    name=instance_data.get("name", "").strip(),
                    stack=instance_data.get("stackName", "").strip(),
                    deploy_status=(instance_data.get("deployStatus") or "").strip(),
                )
                for instance_data in deployment_data.get("instances") or []
            ]

            deployments.append(
                Deployment(
                    id=deployment_data["id"],
                    name=deployment_data.get("name", "").strip(),
                    group=group.get("name", "").strip(),
                    namespace=group.get("namespace", "").strip(),
                    cluster_id=group.get("clusterId"),
                    environment=environment,
                    created_at=parse_timestamp(deployment_data.get("createdAt")),
                    ttl=deployment_data.get("ttl") or 0,
                    instances=instances,
                )
            )

    print(f"Found {len(deployments)} deployments in Instance Manager ({environment})")
    return deployments


def get_all_deployments() -> Tuple[List[Deployment], Dict[Tuple[str, int], str]]:
    env_configs = [(env, os.environ.get(env_var)) for env, env_var in ENVIRONMENTS if os.environ.get(env_var)]

    if not env_configs:
        print("Error: At least one of IM_HOST_PROD or IM_HOST_DEV must be set")
        sys.exit(1)

    deployments = []
    cluster_names = {}

    for environment, im_host in env_configs:
        print(f"Authenticating with Instance Manager ({environment})...")
        access_token = authenticate(im_host, environment)
        cluster_names.update(get_cluster_names(access_token, im_host, environment))
        deployments.extend(get_deployments(access_token, im_host, environment))

    return deployments, cluster_names


def read_cluster(name: str, kubeconfig: str) -> ClusterReport:
    report = ClusterReport(name=name, kubeconfig=kubeconfig)

    try:
        pods = kubectl_json(kubeconfig, "get", "pods", "--all-namespaces", "--selector", INSTANCE_ID_LABEL)
    except ClusterUnreachable as e:
        report.error = str(e)
        return report
    except json.JSONDecodeError as e:
        report.error = f"could not parse the pod listing: {e}"
        return report

    namespaces_by_instance_id = defaultdict(set)
    for pod in pods.get("items") or []:
        metadata = pod["metadata"]
        label = (metadata.get("labels") or {}).get(INSTANCE_ID_LABEL)
        try:
            instance_id = int(label)
        except (TypeError, ValueError):
            continue
        namespaces_by_instance_id[instance_id].add(metadata["namespace"])

    report.namespaces_by_instance_id = dict(namespaces_by_instance_id)
    return report


def read_clusters(cluster_kubeconfigs: Dict[str, str], cluster_names: Dict[Tuple[str, int], str]) -> Dict[str, ClusterReport]:
    print("\nReading pods from the clusters...")

    reports = {}
    for name in sorted(set(cluster_names.values()) | set(cluster_kubeconfigs)):
        kubeconfig = cluster_kubeconfigs.get(name)
        if kubeconfig is None:
            reports[name] = ClusterReport(name=name)
            print(f"  {name}: no kubeconfig given, not checked")
            continue

        report = read_cluster(name, kubeconfig)
        reports[name] = report
        if report.error:
            print(f"  {name}: {report.error}")
        else:
            print(f"  {name}: {len(report.namespaces_by_instance_id)} instances running")

    return reports


def find_missing_workloads(deployments: List[Deployment], reports: Dict[str, ClusterReport], cluster_names: Dict[Tuple[str, int], str]) -> List[MissingWorkloads]:
    missing = []

    for deployment in deployments:
        cluster_name = cluster_names.get((deployment.environment, deployment.cluster_id)) if deployment.cluster_id else None
        report = reports.get(cluster_name) if cluster_name else None
        if report is None or not report.checked:
            continue

        for instance in deployment.instances:
            if instance.deploy_status != DEPLOYED_STATUS:
                continue

            if instance.id not in report.namespaces_by_instance_id:
                missing.append(MissingWorkloads(deployment=deployment, instance=instance))

    return missing


def find_expired_deployments(deployments: List[Deployment], grace: timedelta) -> List[ExpiredDeployment]:
    now = datetime.now(timezone.utc)

    expired = []
    for deployment in deployments:
        expires_at = deployment.expires_at
        if expires_at is None:
            continue

        overdue = now - expires_at
        if overdue > grace:
            expired.append(ExpiredDeployment(deployment=deployment, overdue=overdue))

    return expired


def describe_overdue(overdue: timedelta) -> str:
    days = overdue.days
    hours = overdue.seconds // 3600
    if days:
        return f"{days}d {hours}h"
    return f"{hours}h"


def print_unchecked(reports: Dict[str, ClusterReport], deployments: List[Deployment]):
    unchecked = [report for report in reports.values() if not report.checked]
    clusterless = [deployment for deployment in deployments if not deployment.cluster_id]

    if not unchecked and not clusterless:
        return

    print()
    print(f"Unchecked clusters ({len(unchecked)}):")
    for report in sorted(unchecked, key=lambda report: report.name):
        reason = report.error or "no kubeconfig given"
        print(f"  {report.name}: {reason}")

    if clusterless:
        print(f"  {len(clusterless)} deployment(s) belong to a group with no cluster, so there is no cluster to check them against")

    print("  Instances in these clusters were not checked, so the counts below are incomplete")
    print()


def print_missing_workloads(missing: List[MissingWorkloads]):
    print_separator("Deployed instances with no workloads")

    if not missing:
        print("\nNo deployed instance is missing its workloads")
        return

    print(f"\nDeployed instances with no workloads ({len(missing)}):")
    for finding in sorted(missing, key=lambda finding: (finding.deployment.environment, finding.deployment.group, finding.deployment.name, finding.instance.name)):
        deployment = finding.deployment
        instance = finding.instance
        print(f"  {deployment.environment} / {deployment.group} / {deployment.name} / {instance.name} ({instance.stack}), im-id {instance.id}, namespace {deployment.namespace}")
    print()


def print_expired_deployments(expired: List[ExpiredDeployment]):
    print_separator("Deployments past their TTL")

    if not expired:
        print("\nNo deployment has outlived its TTL")
        return

    print(f"\nDeployments past their TTL ({len(expired)}):")
    for finding in sorted(expired, key=lambda finding: finding.overdue, reverse=True):
        deployment = finding.deployment
        print(f"  {deployment.environment} / {deployment.group} / {deployment.name}, overdue by {describe_overdue(finding.overdue)}, deployment id {deployment.id}")
    print()
    print("A deployment still standing past its TTL is a destroy failing on every inspector pass, so the error is in the Instance Manager log")
    print()


def parse_cluster_arguments(values: List[str]) -> Dict[str, str]:
    kubeconfigs = {}
    for value in values:
        name, separator, path = value.partition("=")
        if not separator or not name.strip() or not path.strip():
            print(f"Error: --cluster expects NAME=PATH, got {value!r}")
            sys.exit(1)
        kubeconfigs[name.strip()] = path.strip()
    return kubeconfigs


def main():
    parser = argparse.ArgumentParser(
        description="Find deployments Instance Manager records as deployed whose workloads are gone, and deployments that have outlived their TTL."
    )
    parser.add_argument(
        "--cluster",
        action="append",
        metavar="NAME=PATH",
        help="An Instance Manager cluster name and the kubeconfig that reaches it (can be used multiple times)",
    )
    parser.add_argument(
        "--ttl-grace-hours",
        type=int,
        default=DEFAULT_TTL_GRACE_HOURS,
        help=f"Ignore deployments whose TTL expired less than this long ago, so a destroy in progress is not reported (default: {DEFAULT_TTL_GRACE_HOURS})",
    )
    args = parser.parse_args()

    cluster_kubeconfigs = parse_cluster_arguments(args.cluster or [])
    if not cluster_kubeconfigs:
        print("Error: At least one --cluster NAME=PATH must be provided")
        sys.exit(1)

    print("Drifted Deployments Check")
    print_separator()

    deployments, cluster_names = get_all_deployments()
    reports = read_clusters(cluster_kubeconfigs, cluster_names)

    missing = find_missing_workloads(deployments, reports, cluster_names)
    expired = find_expired_deployments(deployments, timedelta(hours=args.ttl_grace_hours))

    print_unchecked(reports, deployments)
    print_missing_workloads(missing)
    print_expired_deployments(expired)


if __name__ == "__main__":
    main()
