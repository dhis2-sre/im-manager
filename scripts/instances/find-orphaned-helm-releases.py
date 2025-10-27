#!/usr/bin/env python3

import json
import subprocess
import sys
import os
import argparse
from typing import Set, List, Dict

INSTANCE_MANAGER_CHARTS = {"core", "minio", "postgresql", "pgadmin"}
DEPLOYMENT_SUFFIXES = ["-database", "-minio", "-pgadmin", "-core"]
IM_MANAGER_PREFIX = "im-manager-"


def authenticate() -> str:
    print("Authenticating with Instance Manager...")

    result = subprocess.run(
        ["bash", "-c", f"source ./auth.sh User && echo $ACCESS_TOKEN"],
        capture_output=True, text=True, check=True
    )
    access_token = result.stdout.strip()

    if not access_token:
        print("Failed to obtain access token")
        sys.exit(1)

    print("Authentication successful")
    return access_token


def get_deployments(access_token: str, im_host: str) -> Set[str]:
    print("Fetching Deployments from Instance Manager...")

    result = subprocess.run(
        ["curl", "-s", "-H", f"Authorization: Bearer {access_token}", f"{im_host}/deployments"],
        capture_output=True, text=True, check=True
    )

    data = json.loads(result.stdout)
    deployment_names = {
        deployment["name"]
        for group in data
        for deployment in group.get("deployments", [])
    }

    print(f"Found {len(deployment_names)} deployments in Instance Manager")
    return deployment_names


def get_helm_releases(kubeconfigs: List[str] = None) -> Dict[str, List[Dict]]:
    print("Fetching Helm Releases from Kubernetes...")

    releases_by_cluster = {}
    base_helm_cmd = ["helm", "list", "--all-namespaces", "--max", "1000", "--output", "json"]

    for kubeconfig in kubeconfigs or [None]:
        cluster_name = os.path.basename(kubeconfig) if kubeconfig else "default"
        releases_by_cluster.setdefault(cluster_name, [])

        helm_cmd = base_helm_cmd + (["--kubeconfig", kubeconfig] if kubeconfig else [])
        print(f"  Checking cluster: {cluster_name}")
        result = subprocess.run(helm_cmd, capture_output=True, text=True, check=True)

        processed_releases = process_releases(json.loads(result.stdout))
        releases_by_cluster[cluster_name].extend(processed_releases)

    total_unique = len({
        release["base_name"]
        for releases in releases_by_cluster.values()
        for release in releases
    })
    print(f"Found {total_unique} deployments with Helm releases")
    return releases_by_cluster


def extract_base_name(name: str) -> str:
    for suffix in DEPLOYMENT_SUFFIXES:
        if name.endswith(suffix):
            return name[:-len(suffix)]
    return name


def process_releases(releases: List[Dict]) -> List[Dict]:
    processed_releases = []
    for release in releases:
        name = release["name"]
        chart = release.get("chart", "")
        namespace = release.get("namespace", "")

        if name.startswith(IM_MANAGER_PREFIX):
            continue

        if any(chart.startswith(chart_name) for chart_name in INSTANCE_MANAGER_CHARTS):
            processed_releases.append({
                "name": name,
                "namespace": namespace,
                "base_name": extract_base_name(name)
            })
    return processed_releases


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--kubeconfig", action="append", help="Path to kubeconfig file (can be used multiple times)")
    args = parser.parse_args()

    print("Instance Manager Deployments vs Helm Releases Comparison")
    print("=" * 50)

    kubeconfigs = args.kubeconfig or []
    if kubeconfigs:
        print(f"Using {len(kubeconfigs)} kubeconfig(s): {', '.join(kubeconfigs)}")

    im_host = os.environ.get("IM_HOST")
    if not im_host:
        print("Error: IM_HOST environment variable not set")
        sys.exit(1)

    access_token = authenticate()
    im_deployments = get_deployments(access_token, im_host)
    releases_by_cluster = get_helm_releases(kubeconfigs)
    base_name_releases = {
        release["base_name"]
        for releases in releases_by_cluster.values()
        for release in releases
    }

    print(f"\nInstance Manager Deployments ({len(im_deployments)}):")
    for deployment in sorted(im_deployments):
        print(f"  - {deployment}")

    print(f"\nHelm Releases ({len(base_name_releases)}):")
    for deployment in sorted(base_name_releases):
        print(f"  - {deployment}")

    only_in_helm = base_name_releases - im_deployments

    print(f"\nOrphaned Deployments only in Helm ({len(only_in_helm)}):")
    for cluster_name in sorted(releases_by_cluster.keys()):
        cluster_base_names = {r["base_name"] for r in releases_by_cluster[cluster_name]}
        cluster_orphaned = cluster_base_names & only_in_helm
        if cluster_orphaned:
            print(f"  {cluster_name} ({len(cluster_orphaned)}):")
            for deployment in sorted(cluster_orphaned):
                print(f"    - {deployment}")

    orphaned_helm_commands = generate_helm_delete_commands(only_in_helm, releases_by_cluster, kubeconfigs)
    if orphaned_helm_commands:
        print(f"\n\nHelm commands to delete orphaned releases:")
        for cmd in orphaned_helm_commands:
            print(cmd)


def generate_helm_delete_commands(orphaned_deployments: Set[str], releases_by_cluster: Dict[str, List[Dict]], kubeconfigs: List[str] = None) -> List[str]:
    kubeconfigs = kubeconfigs or []
    cluster_kubeconfigs = {os.path.basename(k): k for k in kubeconfigs}

    commands = []
    for cluster_name, releases in releases_by_cluster.items():
        kubeconfig = cluster_kubeconfigs.get(cluster_name)
        for release in releases:
            if release["base_name"] in orphaned_deployments:
                cmd = f"helm uninstall {release['name']} --namespace {release['namespace']}"
                if kubeconfig:
                    cmd += f" --kubeconfig {kubeconfig}"
                commands.append(cmd)

    return sorted(commands)


if __name__ == "__main__":
    main()