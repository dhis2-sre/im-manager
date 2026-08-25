#!/usr/bin/env python3

"""
Find Orphaned Persistent Volume Claims

This script reports persistent volume claims that nothing owns any more, and how much
Hetzner volume quota is left.

Instance Manager deletes an instance's claims on destroy, per component, by label
selector. A selector that matches nothing returns without error and without logging, so
a wrong selector leaks a volume on every destroy and nothing reports it. That is not
hypothetical: releases gained a group-ID-qualified name in #1410 while the selector kept
using the bare instance name, and by the time #1452 fixed it 289 claims had been left
behind. They filled the Hetzner project's volume quota, and what that looks like from the
outside is not a leak but instances stuck Pending on volumes that cannot be provisioned.

A claim is reported only when all three ownership signals say no:
1. No pod mounts it
2. No workload references it, including a workload scaled to zero, because pausing an
   instance scales it to zero and a paused instance is not an orphan
3. No Helm release matches it, by release annotation or by instance label

With HCLOUD_TOKEN set the script also reports quota headroom and Hetzner volumes with no
persistent volume in any of the given clusters, which the Kubernetes API cannot see at
all. That last comparison is only meaningful when every cluster backed by the Hetzner
project is passed, since a volume belonging to a cluster that was left out looks detached.
"""

import argparse
import json
import os
import subprocess
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Dict, List, Optional, Set, Tuple


DEFAULT_MIN_AGE_HOURS = 6
DEFAULT_QUOTA_GB = 10240
DEFAULT_QUOTA_WARN_PERCENT = 85
HCLOUD_API = "https://api.hetzner.cloud/v1/volumes"
HELM_RELEASE_ANNOTATION = "meta.helm.sh/release-name"
INSTANCE_LABEL = "app.kubernetes.io/instance"


@dataclass
class OrphanedClaim:
    cluster: str
    namespace: str
    name: str
    gibibytes: int
    created: Optional[datetime]
    instance_label: str
    volume_name: str

    @property
    def age_days(self) -> Optional[int]:
        if self.created is None:
            return None
        return (datetime.now(timezone.utc) - self.created).days


@dataclass
class ClusterReport:
    cluster: str
    claims_checked: int = 0
    gibibytes_checked: int = 0
    orphans: List[OrphanedClaim] = field(default_factory=list)
    volume_names: Set[str] = field(default_factory=set)

    @property
    def orphaned_gibibytes(self) -> int:
        return sum(orphan.gibibytes for orphan in self.orphans)


def print_separator(title: str = None):
    if title:
        print("\n" + "=" * 60)
        print(title)
    print("=" * 60)


def kubectl_json(kubeconfig: str, *args: str) -> dict:
    cmd = ["kubectl", "--kubeconfig", kubeconfig, *args, "--output", "json"]
    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode != 0:
        print(f"Error running {' '.join(cmd)}: {result.stderr.strip()}")
        sys.exit(1)

    return json.loads(result.stdout)


def helm_releases(kubeconfig: str) -> Set[Tuple[str, str]]:
    cmd = ["helm", "list", "--kubeconfig", kubeconfig, "--all-namespaces", "--all", "--output", "json"]
    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode != 0:
        print(f"Error running {' '.join(cmd)}: {result.stderr.strip()}")
        sys.exit(1)

    return {(release["namespace"], release["name"]) for release in json.loads(result.stdout)}


def parse_timestamp(value: str) -> Optional[datetime]:
    try:
        return datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    except (TypeError, ValueError):
        return None


def parse_gibibytes(value: str) -> int:
    for suffix, multiplier in (("Ti", 1024), ("Gi", 1), ("Mi", 0)):
        if value.endswith(suffix):
            return int(float(value[: -len(suffix)]) * multiplier)
    return 0


def claims_in_pod_spec(namespace: str, pod_spec: dict) -> Set[Tuple[str, str]]:
    claims = set()
    for volume in pod_spec.get("volumes") or []:
        claim = volume.get("persistentVolumeClaim")
        if claim and claim.get("claimName"):
            claims.add((namespace, claim["claimName"]))
    return claims


def get_mounted_claims(kubeconfig: str) -> Set[Tuple[str, str]]:
    """Claims mounted by a pod that exists right now."""
    pods = kubectl_json(kubeconfig, "get", "pods", "--all-namespaces")
    mounted = set()
    for pod in pods["items"]:
        mounted |= claims_in_pod_spec(pod["metadata"]["namespace"], pod["spec"])
    return mounted


def get_referenced_claims(kubeconfig: str) -> Set[Tuple[str, str]]:
    """Claims a workload references even with no pod running, so pausing is not an orphan.

    StatefulSet volume claim templates are matched by name across every ordinal rather
    than up to the current replica count, because scaling down deliberately keeps the
    claims of the ordinals it removed.
    """
    referenced = set()

    workloads = kubectl_json(
        kubeconfig, "get", "deployments,statefulsets,daemonsets,jobs", "--all-namespaces"
    )
    for workload in workloads["items"]:
        namespace = workload["metadata"]["namespace"]
        referenced |= claims_in_pod_spec(namespace, workload["spec"]["template"]["spec"])

    cronjobs = kubectl_json(kubeconfig, "get", "cronjobs", "--all-namespaces")
    for cronjob in cronjobs["items"]:
        namespace = cronjob["metadata"]["namespace"]
        pod_spec = cronjob["spec"]["jobTemplate"]["spec"]["template"]["spec"]
        referenced |= claims_in_pod_spec(namespace, pod_spec)

    return referenced


def get_template_claim_prefixes(kubeconfig: str) -> Set[Tuple[str, str]]:
    statefulsets = kubectl_json(kubeconfig, "get", "statefulsets", "--all-namespaces")
    prefixes = set()
    for statefulset in statefulsets["items"]:
        namespace = statefulset["metadata"]["namespace"]
        name = statefulset["metadata"]["name"]
        for template in statefulset["spec"].get("volumeClaimTemplates") or []:
            prefixes.add((namespace, f"{template['metadata']['name']}-{name}-"))
    return prefixes


def owned_by_statefulset_template(namespace: str, name: str, prefixes: Set[Tuple[str, str]]) -> bool:
    for template_namespace, prefix in prefixes:
        if namespace != template_namespace or not name.startswith(prefix):
            continue
        if name[len(prefix):].isdigit():
            return True
    return False


def check_cluster(kubeconfig: str, min_age_hours: int, ignored_namespaces: Set[str]) -> ClusterReport:
    report = ClusterReport(cluster=os.path.basename(kubeconfig))

    mounted = get_mounted_claims(kubeconfig)
    referenced = get_referenced_claims(kubeconfig)
    template_prefixes = get_template_claim_prefixes(kubeconfig)
    releases = helm_releases(kubeconfig)

    claims = kubectl_json(kubeconfig, "get", "pvc", "--all-namespaces")
    now = datetime.now(timezone.utc)

    for claim in claims["items"]:
        metadata = claim["metadata"]
        namespace = metadata["namespace"]
        name = metadata["name"]
        if namespace in ignored_namespaces:
            continue

        gibibytes = parse_gibibytes(claim["spec"]["resources"]["requests"]["storage"])
        report.claims_checked += 1
        report.gibibytes_checked += gibibytes

        volume_name = claim["spec"].get("volumeName") or ""
        if volume_name:
            report.volume_names.add(volume_name)

        if (namespace, name) in mounted or (namespace, name) in referenced:
            continue
        if owned_by_statefulset_template(namespace, name, template_prefixes):
            continue

        annotations = metadata.get("annotations") or {}
        labels = metadata.get("labels") or {}
        instance_label = labels.get(INSTANCE_LABEL, "")
        release = annotations.get(HELM_RELEASE_ANNOTATION) or instance_label
        if release and (namespace, release) in releases:
            continue

        created = parse_timestamp(metadata.get("creationTimestamp"))
        if created is not None and (now - created).total_seconds() < min_age_hours * 3600:
            continue

        report.orphans.append(
            OrphanedClaim(
                cluster=report.cluster,
                namespace=namespace,
                name=name,
                gibibytes=gibibytes,
                created=created,
                instance_label=instance_label,
                volume_name=volume_name,
            )
        )

    return report


def hcloud_volumes(token: str) -> List[dict]:
    volumes = []
    page = 1

    while True:
        request = urllib.request.Request(
            f"{HCLOUD_API}?page={page}&per_page=50",
            headers={"Authorization": f"Bearer {token}"},
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                payload = json.load(response)
        except (urllib.error.URLError, urllib.error.HTTPError) as error:
            print(f"Warning: could not read Hetzner volumes: {error}")
            return []

        volumes.extend(payload.get("volumes", []))
        next_page = (payload.get("meta", {}).get("pagination") or {}).get("next_page")
        if not next_page:
            return volumes
        page = next_page


def print_orphans(reports: List[ClusterReport]):
    orphans = [orphan for report in reports for orphan in report.orphans]
    if not orphans:
        print("\nNo orphaned persistent volume claims found")
        return

    orphans.sort(key=lambda orphan: (orphan.age_days is None, -(orphan.age_days or 0)))
    total = sum(orphan.gibibytes for orphan in orphans)

    print()
    print(f"Orphaned persistent volume claims ({len(orphans)}): {total} Gi")
    for orphan in orphans:
        age = f"{orphan.age_days} days old" if orphan.age_days is not None else "age unknown"
        instance = f", instance {orphan.instance_label}" if orphan.instance_label else ""
        print(f"  {orphan.cluster} / {orphan.namespace} / {orphan.name}: {orphan.gibibytes} Gi, {age}{instance}")
    print()


def print_quota(reports: List[ClusterReport], volumes: List[dict], quota_gb: int, warn_percent: int):
    if not volumes:
        return

    provisioned = sum(volume["size"] for volume in volumes)
    used_percent = int(provisioned * 100 / quota_gb) if quota_gb else 0

    known = set()
    for report in reports:
        known |= report.volume_names
    detached = [volume for volume in volumes if volume["name"] not in known]

    print_separator("Hetzner volume quota")
    print(f"Provisioned: {provisioned} GB of {quota_gb} GB across {len(volumes)} volume(s), {used_percent}% used")

    if used_percent >= warn_percent:
        print()
        print(f"Volume quota nearly exhausted ({used_percent}%): {quota_gb - provisioned} GB left")
        print("  New persistent volume claims fail with 'volumes size limit exceeded' once the quota is reached")
        print()

    if detached:
        detached_size = sum(volume["size"] for volume in detached)
        print()
        print(f"Hetzner volumes with no persistent volume ({len(detached)}): {detached_size} GB")
        print("  Assumes every cluster using this Hetzner project was passed, otherwise another cluster's volumes land here")
        for volume in sorted(detached, key=lambda volume: volume["created"]):
            labels = volume.get("labels") or {}
            claim = labels.get("pvc-name", "unknown")
            namespace = labels.get("pvc-namespace", "unknown")
            print(f"  {volume['name']}: {volume['size']} GB, created {volume['created'][:10]}, was {namespace}/{claim}")
        print()


def generate_remediation_commands(reports: List[ClusterReport], kubeconfigs: List[str], volumes: List[dict]) -> List[str]:
    commands = []
    kubeconfig_map = {os.path.basename(kubeconfig): kubeconfig for kubeconfig in kubeconfigs}

    for report in reports:
        kubeconfig_path = kubeconfig_map.get(report.cluster)
        for orphan in report.orphans:
            cmd = f"kubectl --namespace {orphan.namespace} delete pvc {orphan.name}"
            if kubeconfig_path:
                cmd += f" --kubeconfig {kubeconfig_path}"
            commands.append(cmd)

    known = {name for report in reports for name in report.volume_names}
    for volume in volumes:
        if volume["name"] not in known:
            commands.append(f"hcloud volume delete {volume['name']}")

    return commands


def main():
    parser = argparse.ArgumentParser(
        description="Find persistent volume claims that no pod, workload or Helm release owns, and report Hetzner volume quota headroom."
    )
    parser.add_argument("--kubeconfig", action="append", help="Path to kubeconfig file (can be used multiple times)")
    parser.add_argument(
        "--min-age-hours",
        type=int,
        default=DEFAULT_MIN_AGE_HOURS,
        help=f"Ignore claims younger than this, so a deploy in progress is not reported (default: {DEFAULT_MIN_AGE_HOURS})",
    )
    parser.add_argument("--ignore-namespace", action="append", help="Namespace to skip (can be used multiple times)")
    parser.add_argument(
        "--quota-gb",
        type=int,
        default=DEFAULT_QUOTA_GB,
        help=f"Hetzner project volume size quota in GB (default: {DEFAULT_QUOTA_GB})",
    )
    parser.add_argument(
        "--quota-warn-percent",
        type=int,
        default=DEFAULT_QUOTA_WARN_PERCENT,
        help=f"Report the quota when this percentage is used (default: {DEFAULT_QUOTA_WARN_PERCENT})",
    )
    args = parser.parse_args()

    kubeconfigs = args.kubeconfig or []
    if not kubeconfigs:
        print("Error: At least one --kubeconfig must be provided")
        sys.exit(1)

    ignored_namespaces = set(args.ignore_namespace or [])

    print("Orphaned Persistent Volume Claims Check")
    print_separator()
    print(f"Using {len(kubeconfigs)} kubeconfig(s): {', '.join(kubeconfigs)}")
    if ignored_namespaces:
        print(f"Ignoring namespace(s): {', '.join(sorted(ignored_namespaces))}")
    print()

    print("Fetching persistent volume claims, workloads and Helm releases from Kubernetes...")
    reports = [check_cluster(kubeconfig, args.min_age_hours, ignored_namespaces) for kubeconfig in kubeconfigs]

    checked = sum(report.claims_checked for report in reports)
    checked_gibibytes = sum(report.gibibytes_checked for report in reports)
    print(f"Found {checked} persistent volume claims totalling {checked_gibibytes} Gi")
    for report in reports:
        print(f"  {report.cluster}: {report.claims_checked} claims, {report.gibibytes_checked} Gi, {len(report.orphans)} orphaned")

    print_orphans(reports)

    token = os.environ.get("HCLOUD_TOKEN", "")
    volumes = hcloud_volumes(token) if token else []
    if not token:
        print("HCLOUD_TOKEN is not set, skipping the Hetzner volume quota check")
    else:
        print_quota(reports, volumes, args.quota_gb, args.quota_warn_percent)

    commands = generate_remediation_commands(reports, kubeconfigs, volumes)
    if commands:
        print(f"\n\nCommands to remove orphaned volumes ({len(commands)}):")
        for cmd in commands:
            print(cmd)


if __name__ == "__main__":
    main()
