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

Check 1 compares against every kubeconfig given, and a kubeconfig that cannot be read
withdraws the check rather than narrowing it. An unreadable cluster looks exactly like a
cluster running nothing, so continuing would report every instance on it at once, and an
alarm that cries wolf once is an alarm nobody reads again.

That leaves instances on a cluster no kubeconfig reaches, which would be reported as
having lost workloads that are in fact running somewhere this script cannot see. Rather
than resolve each deployment's cluster to a kubeconfig, which would mean naming clusters
here, findings are grouped by the cluster id Instance Manager holds, so a cluster that is
not covered stands out as a block of findings sharing one id. --ignore-cluster-id drops
it until its kubeconfig is added.
"""

import argparse
import json
import os
import shlex
import subprocess
import sys
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import List, Optional, Set

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
    withdraws check 1 entirely, because an unreadable cluster is indistinguishable from one
    running nothing."""

    kubeconfig: str
    instance_ids: Set[int] = field(default_factory=set)
    error: Optional[str] = None

    @property
    def name(self) -> str:
        return os.path.basename(self.kubeconfig)


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


def get_all_deployments() -> List[Deployment]:
    env_configs = [(env, os.environ.get(env_var)) for env, env_var in ENVIRONMENTS if os.environ.get(env_var)]

    if not env_configs:
        print("Error: At least one of IM_HOST_PROD or IM_HOST_DEV must be set")
        sys.exit(1)

    deployments = []

    for environment, im_host in env_configs:
        print(f"Authenticating with Instance Manager ({environment})...")
        access_token = authenticate(im_host, environment)
        deployments.extend(get_deployments(access_token, im_host, environment))

    return deployments


def read_cluster(kubeconfig: str) -> ClusterReport:
    report = ClusterReport(kubeconfig=kubeconfig)

    try:
        pods = kubectl_json(kubeconfig, "get", "pods", "--all-namespaces", "--selector", INSTANCE_ID_LABEL)
    except ClusterUnreachable as e:
        report.error = str(e)
        return report
    except json.JSONDecodeError as e:
        report.error = f"could not parse the pod listing: {e}"
        return report

    for pod in pods.get("items") or []:
        label = (pod["metadata"].get("labels") or {}).get(INSTANCE_ID_LABEL)
        try:
            report.instance_ids.add(int(label))
        except (TypeError, ValueError):
            continue

    return report


def read_clusters(kubeconfigs: List[str]) -> List[ClusterReport]:
    print("\nReading pods from the clusters...")

    reports = []
    for kubeconfig in kubeconfigs:
        report = read_cluster(kubeconfig)
        reports.append(report)
        if report.error:
            print(f"  {report.name}: {report.error}")
        else:
            print(f"  {report.name}: {len(report.instance_ids)} instances running")

    return reports


def find_missing_workloads(deployments: List[Deployment], reports: List[ClusterReport], ignored_cluster_ids: Set[int]) -> List[MissingWorkloads]:
    """Instances whose im-id no checked cluster is running. Withdrawn entirely when any
    kubeconfig failed, since the instances that cluster was running cannot be told apart from
    the ones that really have lost their workloads."""
    if any(report.error for report in reports):
        return []

    running = set().union(*(report.instance_ids for report in reports)) if reports else set()

    missing = []
    for deployment in deployments:
        if deployment.cluster_id in ignored_cluster_ids:
            continue

        for instance in deployment.instances:
            if instance.deploy_status != DEPLOYED_STATUS:
                continue

            if instance.id not in running:
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


def print_unchecked(reports: List[ClusterReport]):
    unreadable = [report for report in reports if report.error]
    if not unreadable:
        return

    print()
    print(f"Unreadable clusters ({len(unreadable)}):")
    for report in unreadable:
        print(f"  {report.name}: {report.error}")
    print("  A cluster that cannot be read looks the same as a cluster running nothing, so the workload check is withdrawn rather than reporting every instance at once")
    print()


def print_missing_workloads(missing: List[MissingWorkloads]):
    print_separator("Deployed instances with no workloads")

    if not missing:
        print("\nNo deployed instance is missing its workloads")
        return

    print(f"\nDeployed instances with no workloads ({len(missing)}):")
    for finding in sorted(missing, key=lambda finding: (finding.deployment.cluster_id or 0, finding.deployment.environment, finding.deployment.group, finding.deployment.name, finding.instance.name)):
        deployment = finding.deployment
        instance = finding.instance
        cluster = f"cluster {deployment.cluster_id}" if deployment.cluster_id else "no cluster"
        print(f"  {deployment.environment} / {deployment.group} / {deployment.name} / {instance.name} ({instance.stack}), im-id {instance.id}, namespace {deployment.namespace}, {cluster}")
    print()
    print("Findings sharing one cluster id are likely a cluster no kubeconfig here reaches, whose instances are running where this check cannot see them. Add its kubeconfig, or pass --ignore-cluster-id to drop it.")
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


def main():
    parser = argparse.ArgumentParser(
        description="Find deployments Instance Manager records as deployed whose workloads are gone, and deployments that have outlived their TTL."
    )
    parser.add_argument("--kubeconfig", action="append", help="Path to kubeconfig file (can be used multiple times)")
    parser.add_argument(
        "--ignore-cluster-id",
        action="append",
        type=int,
        help="Skip deployments on this Instance Manager cluster, for a cluster no kubeconfig here reaches (can be used multiple times)",
    )
    parser.add_argument(
        "--ttl-grace-hours",
        type=int,
        default=DEFAULT_TTL_GRACE_HOURS,
        help=f"Ignore deployments whose TTL expired less than this long ago, so a destroy in progress is not reported (default: {DEFAULT_TTL_GRACE_HOURS})",
    )
    args = parser.parse_args()

    kubeconfigs = args.kubeconfig or []
    if not kubeconfigs:
        print("Error: At least one --kubeconfig must be provided")
        sys.exit(1)

    ignored_cluster_ids = set(args.ignore_cluster_id or [])

    print("Drifted Deployments Check")
    print_separator()

    deployments = get_all_deployments()
    reports = read_clusters(kubeconfigs)

    missing = find_missing_workloads(deployments, reports, ignored_cluster_ids)
    expired = find_expired_deployments(deployments, timedelta(hours=args.ttl_grace_hours))

    print_unchecked(reports)
    print_missing_workloads(missing)
    print_expired_deployments(expired)


if __name__ == "__main__":
    main()
