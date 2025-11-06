#!/usr/bin/env python3

"""
Find Orphaned Helm Releases

This script compares Instance Manager Deployments with Helm Releases
in Kubernetes clusters to identify orphaned Helm Releases that no longer exist
in the Instance Manager.

The script:
1. Fetches all deployments from Instance Manager (prod and/or dev environments)
2. Fetches all Helm releases from specified Kubernetes clusters
3. Compares them to find Releases that exist in Helm but not in Instance Manager
4. Outputs Helm uninstall commands for the orphaned releases
"""

import json
import os
import shlex
import subprocess
import sys
import argparse
import requests
from collections import defaultdict
from dataclasses import dataclass
from typing import List, Dict, Optional


INSTANCE_MANAGER_CHARTS = {"core", "minio", "postgresql", "pgadmin"}
DEPLOYMENT_SUFFIXES = ["-database", "-minio", "-pgadmin"]
IM_MANAGER_PREFIX = "im-manager-"
ENVIRONMENTS = [("prod", "IM_HOST_PROD"), ("dev", "IM_HOST_DEV")]
CLUSTERS_MAPPING = {
    "hetzner": "hetzner",
}


@dataclass
class Cluster:
    id: int
    name: str


@dataclass
class Group:
    name: str
    hostname: str
    cluster_id: Optional[int]
    namespace: str


@dataclass
class Deployment:
    name: str
    namespace: str
    cluster: str
    group: str
    environment: str


@dataclass
class HelmRelease:
    name: str
    namespace: str
    chart: str
    base_name: str


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
        if result.stdout:
            print(f"Output: {result.stdout}")
        sys.exit(1)

    return access_token


def normalize_cluster_name(cluster_name: str) -> str:
    cluster_name_lower = cluster_name.lower()
    for key, normalized in CLUSTERS_MAPPING.items():
        if key in cluster_name_lower:
            return normalized
    return "default"


def print_separator(title: str = None):
    if title:
        print("\n" + "=" * 60)
        print(title)
    print("=" * 60)


def get_clusters(access_token: str, im_host: str) -> Dict[int, Cluster]:
    try:
        response = requests.get(
            f"{im_host}/clusters",
            headers={"Authorization": f"Bearer {access_token}"},
            timeout=30
        )
        response.raise_for_status()
    except requests.exceptions.RequestException as e:
        print(f"Error fetching clusters from {im_host}/clusters: {e}")
        response = getattr(e, 'response', None)
        if response is not None:
            print(f"Response status: {response.status_code}")
            print(f"Response body: {response.text}")
        sys.exit(1)

    if not response.text.strip():
        return {}

    clusters_data = response.json()

    cluster_map = {
        cluster_data["id"]: Cluster(id=cluster_data["id"], name=cluster_data["name"].strip())
        for cluster_data in clusters_data
    }

    return cluster_map


def get_deployments_by_environment(access_token: str, im_host: str, environment: str, cluster_map: Dict[int, Cluster]) -> List[Deployment]:
    print(f"Fetching Deployments from Instance Manager ({environment})...")

    try:
        response = requests.get(
            f"{im_host}/deployments",
            headers={"Authorization": f"Bearer {access_token}"},
            timeout=30
        )
        response.raise_for_status()
    except requests.exceptions.RequestException as e:
        print(f"Error fetching deployments from {im_host}/deployments: {e}")
        response = getattr(e, 'response', None)
        if response is not None:
            print(f"Response status: {response.status_code}")
            print(f"Response body: {response.text}")
        sys.exit(1)

    if not response.text.strip():
        print(f"Error: Empty response from {im_host}/deployments")
        sys.exit(1)

    groups_data = response.json()

    deployments = [
        Deployment(
            name=deployment_data["name"].strip(),
            namespace=deployment_data["group"]["namespace"].strip(),
            cluster=normalize_cluster_name(cluster_map[deployment_data["group"]["clusterId"]].name)
            if (cluster_id := deployment_data["group"].get("clusterId")) and cluster_id in cluster_map
            else "default",
            group=group_data.get("name", "").strip(),
            environment=environment
        )
        for group_data in groups_data
        for deployment_data in group_data["deployments"]
    ]

    print(f"Found {len(deployments)} deployments in Instance Manager ({environment})\n")
    return deployments


def extract_base_name(name: str) -> str:
    for suffix in DEPLOYMENT_SUFFIXES:
        if name.endswith(suffix):
            return name[:-len(suffix)]
    return name


def get_helm_releases(kubeconfigs: List[str]) -> Dict[str, Dict[str, List[HelmRelease]]]:
    print("Fetching Helm Releases from Kubernetes...")

    releases_by_kubeconfig = defaultdict(lambda: defaultdict(list))
    base_helm_cmd = ["helm", "list", "--all-namespaces", "--max", "1000", "--output", "json"]

    for kubeconfig in kubeconfigs:
        kubeconfig_name = os.path.basename(kubeconfig)
        helm_cmd = base_helm_cmd + ["--kubeconfig", kubeconfig]
        print(f"  Checking kubeconfig: {kubeconfig_name}")
        result = subprocess.run(helm_cmd, capture_output=True, text=True, check=True)

        releases_data = json.loads(result.stdout)

        for release_data in releases_data:
            release_name = release_data["name"]
            if release_name.startswith(IM_MANAGER_PREFIX):
                continue

            chart = release_data["chart"]
            if any(chart.startswith(chart_name) for chart_name in INSTANCE_MANAGER_CHARTS):
                namespace = release_data["namespace"]
                releases_by_kubeconfig[kubeconfig_name][namespace].append(
                    HelmRelease(
                        name=release_name,
                        namespace=namespace,
                        chart=chart,
                        base_name=extract_base_name(release_name)
                    )
                )

    total = sum(len(releases) for namespaces in releases_by_kubeconfig.values() for releases in namespaces.values())
    print(f"Found {total} Helm Releases")
    return releases_by_kubeconfig


def get_all_deployments() -> Dict[str, List[Deployment]]:
    env_configs = [(env, os.environ.get(env_var)) for env, env_var in ENVIRONMENTS if os.environ.get(env_var)]

    if not env_configs:
        print("Error: At least one of IM_HOST_PROD or IM_HOST_DEV must be set")
        sys.exit(1)

    deployments_by_env = {}
    cluster_map = {}

    for env, im_host in env_configs:
        print(f"Authenticating with Instance Manager ({env})...")
        access_token = authenticate(im_host, env)
        print(f"Authentication successful ({env})")
        cluster_map.update(get_clusters(access_token, im_host))
        deployments_by_env[env] = get_deployments_by_environment(access_token, im_host, env, cluster_map)

    return deployments_by_env


def print_deployments(deployments_by_env: Dict[str, List[Deployment]]):
    print_separator("Instance Manager Deployments")

    for env in sorted(deployments_by_env.keys()):
        deployments = deployments_by_env[env]
        print(f"\n{env.upper()} ({len(deployments)} deployments):")

        by_group = defaultdict(lambda: {"namespace": "", "deployments": []})
        for deployment in deployments:
            group_name = deployment.group
            by_group[group_name]["namespace"] = deployment.namespace
            by_group[group_name]["deployments"].append(deployment.name)

        for group_name in sorted(by_group.keys()):
            group_data = by_group[group_name]
            print(f"  {group_name} (namespace: {group_data['namespace']}) ({len(group_data['deployments'])}):")
            for name in sorted(group_data["deployments"]):
                print(f"    - {name}")


def print_helm_releases(helm_releases_by_kubeconfig: Dict[str, Dict[str, List[HelmRelease]]]):
    print_separator("Helm Releases")

    for kubeconfig_name in sorted(helm_releases_by_kubeconfig.keys()):
        namespaces = helm_releases_by_kubeconfig[kubeconfig_name]
        unique_releases = {(namespace, release.base_name) for namespace, releases in namespaces.items() for release in releases}
        print(f"\n{kubeconfig_name} ({len(unique_releases)} releases):")

        for namespace in sorted(namespaces.keys()):
            base_names = sorted({r.base_name for r in namespaces[namespace]})
            print(f"  {namespace} ({len(base_names)}):")
            for base_name in base_names:
                print(f"    - {base_name}")


def find_orphaned_releases(
    deployments_by_env: Dict[str, List[Deployment]],
    helm_releases_by_kubeconfig: Dict[str, Dict[str, List[HelmRelease]]]
) -> Dict[str, Dict[str, List[HelmRelease]]]:
    im_deployments_by_cluster = defaultdict(set)
    for deployments in deployments_by_env.values():
        for deployment in deployments:
            im_deployments_by_cluster[deployment.cluster].add((deployment.namespace, deployment.name))

    helm_releases_by_cluster = defaultdict(set)
    kubeconfig_to_cluster = {}
    for kubeconfig_name, namespaces in helm_releases_by_kubeconfig.items():
        cluster = normalize_cluster_name(kubeconfig_name)
        kubeconfig_to_cluster[kubeconfig_name] = cluster

        for namespace, releases in namespaces.items():
            for release in releases:
                helm_releases_by_cluster[cluster].add((namespace, release.base_name))

    orphaned_by_kubeconfig = {}
    for kubeconfig_name, namespaces in helm_releases_by_kubeconfig.items():
        cluster = kubeconfig_to_cluster[kubeconfig_name]
        orphaned_keys = helm_releases_by_cluster[cluster] - im_deployments_by_cluster.get(cluster, set())

        if orphaned_keys:
            orphaned_by_kubeconfig[kubeconfig_name] = {}
            for namespace, releases in namespaces.items():
                orphaned_releases = [release for release in releases if (namespace, release.base_name) in orphaned_keys]
                if orphaned_releases:
                    orphaned_by_kubeconfig[kubeconfig_name][namespace] = orphaned_releases

    return orphaned_by_kubeconfig


def print_orphaned_releases(orphaned_by_kubeconfig: Dict[str, Dict[str, List[HelmRelease]]]):
    total = sum(
        len({r.base_name for r in releases})
        for namespaces in orphaned_by_kubeconfig.values()
        for releases in namespaces.values()
    )

    print_separator("Comparison")
    print(f"\nOrphaned Deployments only in Helm ({total}):")

    for kubeconfig_name in sorted(orphaned_by_kubeconfig.keys()):
        namespaces = orphaned_by_kubeconfig[kubeconfig_name]
        for namespace in sorted(namespaces.keys()):
            releases = namespaces[namespace]
            base_names = sorted({release.base_name for release in releases})
            print(f"  {kubeconfig_name} / {namespace} ({len(base_names)}):")
            for base_name in base_names:
                print(f"    - {base_name}")


def generate_helm_commands(orphaned_by_kubeconfig: Dict[str, Dict[str, List[HelmRelease]]], kubeconfigs: List[str]) -> List[str]:
    commands = []
    kubeconfig_map = {os.path.basename(kubeconfig): kubeconfig for kubeconfig in kubeconfigs}

    for kubeconfig_name, namespaces in orphaned_by_kubeconfig.items():
        kubeconfig_path = kubeconfig_map.get(kubeconfig_name)
        for _, releases in namespaces.items():
            for release in releases:
                cmd = f"helm uninstall {release.name} --namespace {release.namespace}"
                if kubeconfig_path:
                    cmd += f" --kubeconfig {kubeconfig_path}"
                commands.append(cmd)

    return sorted(commands)


def main():
    parser = argparse.ArgumentParser(
        description="Find orphaned Helm releases by comparing Instance Manager deployments with Kubernetes Helm releases."
    )
    parser.add_argument("--kubeconfig", action="append", help="Path to kubeconfig file (can be used multiple times)")
    args = parser.parse_args()

    kubeconfigs = args.kubeconfig or []
    if not kubeconfigs:
        print("Error: At least one --kubeconfig must be provided")
        sys.exit(1)

    print("Instance Manager Deployments vs Helm Releases Comparison")
    print_separator()
    print(f"Using {len(kubeconfigs)} kubeconfig(s): {', '.join(kubeconfigs)}\n")

    deployments_by_env = get_all_deployments()
    helm_releases_by_kubeconfig = get_helm_releases(kubeconfigs)

    print_deployments(deployments_by_env)
    print_helm_releases(helm_releases_by_kubeconfig)

    orphaned_by_kubeconfig = find_orphaned_releases(deployments_by_env, helm_releases_by_kubeconfig)
    print_orphaned_releases(orphaned_by_kubeconfig)

    commands = generate_helm_commands(orphaned_by_kubeconfig, kubeconfigs)
    if commands:
        print(f"\n\nHelm commands to delete orphaned releases ({len(commands)}):")
        for cmd in commands:
            print(cmd)


if __name__ == "__main__":
    main()

