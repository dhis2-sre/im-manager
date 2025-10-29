#!/usr/bin/env python3

import json
import os
import shlex
import subprocess
import sys
import argparse
from collections import defaultdict
from typing import List, Dict

INSTANCE_MANAGER_CHARTS = {"core", "minio", "postgresql", "pgadmin"}
DEPLOYMENT_SUFFIXES = ["-database", "-minio", "-pgadmin"]
IM_MANAGER_PREFIX = "im-manager-"
ENVIRONMENTS = [("prod", "IM_HOST_PROD"), ("dev", "IM_HOST_DEV")]


def authenticate(im_host: str, environment: str) -> str:
    user_type = f"User_{environment}"
    user_email = os.environ.get(f"USER_EMAIL_{environment.upper()}")
    password = os.environ.get(f"PASSWORD_{environment.upper()}")

    if not user_email or not password:
        print(f"Error: Credentials for {environment} environment not found. Set USER_EMAIL_{environment.upper()} and PASSWORD_{environment.upper()}")
        sys.exit(1)

    cmd = (
        f"export IM_HOST={shlex.quote(im_host)} && "
        f"export USER_EMAIL={shlex.quote(user_email)} && "
        f"export PASSWORD={shlex.quote(password)} && "
        f"source ./auth.sh {shlex.quote(user_type)} && echo $ACCESS_TOKEN"
    )

    result = subprocess.run(["bash", "-c", cmd], capture_output=True, text=True, check=True)
    access_token = result.stdout.strip()

    if not access_token:
        print(f"Failed to obtain access token for {im_host}")
        sys.exit(1)

    return access_token


def normalize_cluster_name(cluster_name: str) -> str:
    return "hetzner" if "hetzner" in cluster_name.lower() else "default"


def print_separator(title: str = None):
    if title:
        print("\n" + "=" * 60)
        print(title)
    print("=" * 60)


def get_clusters(access_token: str, im_host: str) -> Dict[int, str]:
    result = subprocess.run(
        ["curl", "-s", "-H", f"Authorization: Bearer {access_token}", f"{im_host}/clusters"],
        capture_output=True, text=True, check=True
    )

    if not result.stdout.strip():
        return {}

    cluster_map = {}
    for cluster in json.loads(result.stdout):
        cluster_id = cluster.get("id")
        cluster_name = cluster.get("name", "").strip()
        if cluster_id and cluster_name:
            cluster_map[cluster_id] = normalize_cluster_name(cluster_name)

    return cluster_map


def get_deployments_by_environment(access_token: str, im_host: str, environment: str, cluster_map: Dict[int, str]) -> List[Dict]:
    print(f"Fetching Deployments from Instance Manager ({environment})...")

    result = subprocess.run(
        ["curl", "-s", "-H", f"Authorization: Bearer {access_token}", f"{im_host}/deployments"],
        capture_output=True, text=True, check=True
    )

    if not result.stdout.strip():
        print(f"Error: Empty response from {im_host}/deployments")
        sys.exit(1)

    deployments = []
    for group in json.loads(result.stdout):
        group_name = group["name"]
        for deployment in group.get("deployments", []):
            deployment_name = deployment.get("name", "").strip()
            if not deployment_name:
                continue

            deployment_group = deployment.get("group")
            cluster_id = deployment_group.get("clusterId")
            cluster_name = cluster_map.get(cluster_id, "default") if cluster_id else "default"

            deployments.append({
                "name": deployment_name,
                "namespace": deployment_group.get("namespace", "").strip(),
                "cluster": cluster_name,
                "group": group_name,
                "environment": environment
            })

    print(f"Found {len(deployments)} deployments in Instance Manager ({environment})\n")
    return deployments


def extract_base_name(name: str) -> str:
    for suffix in DEPLOYMENT_SUFFIXES:
        if name.endswith(suffix):
            return name[:-len(suffix)]
    return name


def get_helm_releases(kubeconfigs: List[str]) -> Dict[str, Dict[str, List[Dict]]]:
    print("Fetching Helm Releases from Kubernetes...")

    releases_by_kubeconfig = defaultdict(lambda: defaultdict(list))
    base_helm_cmd = ["helm", "list", "--all-namespaces", "--max", "1000", "--output", "json"]

    for kubeconfig in kubeconfigs:
        kubeconfig_name = os.path.basename(kubeconfig)
        helm_cmd = base_helm_cmd + ["--kubeconfig", kubeconfig]
        print(f"  Checking kubeconfig: {kubeconfig_name}")
        result = subprocess.run(helm_cmd, capture_output=True, text=True, check=True)

        for release in json.loads(result.stdout):
            release_name = release.get("name", "")
            if release_name.startswith(IM_MANAGER_PREFIX):
                continue

            chart = release.get("chart", "")
            if any(chart.startswith(chart_name) for chart_name in INSTANCE_MANAGER_CHARTS):
                namespace = release.get("namespace", "")
                releases_by_kubeconfig[kubeconfig_name][namespace].append({
                    "name": release_name,
                    "namespace": namespace,
                    "base_name": extract_base_name(release_name)
                })

    total = sum(len(releases) for namespaces in releases_by_kubeconfig.values() for releases in namespaces.values())
    print(f"Found {total} Helm Releases")
    return releases_by_kubeconfig


def get_all_deployments() -> Dict[str, List[Dict]]:
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


def print_deployments(deployments_by_env: Dict[str, List[Dict]]):
    print_separator("Instance Manager Deployments")

    for env in sorted(deployments_by_env.keys()):
        deployments = deployments_by_env[env]
        print(f"\n{env.upper()} ({len(deployments)} deployments):")

        by_group = defaultdict(lambda: {"namespace": "", "deployments": []})
        for deployment in deployments:
            group_name = deployment["group"]
            by_group[group_name]["namespace"] = deployment["namespace"]
            by_group[group_name]["deployments"].append(deployment["name"])

        for group_name in sorted(by_group.keys()):
            group_data = by_group[group_name]
            print(f"  {group_name} (namespace: {group_data['namespace']}) ({len(group_data['deployments'])}):")
            for name in sorted(group_data["deployments"]):
                print(f"    - {name}")


def print_helm_releases(helm_releases_by_kubeconfig: Dict[str, Dict[str, List[Dict]]]):
    print_separator("Helm Releases")

    for kubeconfig_name in sorted(helm_releases_by_kubeconfig.keys()):
        namespaces = helm_releases_by_kubeconfig[kubeconfig_name]
        unique_releases = {(namespace, release["base_name"]) for namespace, releases in namespaces.items() for release in releases}
        print(f"\n{kubeconfig_name} ({len(unique_releases)} releases):")

        for namespace in sorted(namespaces.keys()):
            base_names = sorted({r["base_name"] for r in namespaces[namespace]})
            print(f"  {namespace} ({len(base_names)}):")
            for base_name in base_names:
                print(f"    - {base_name}")


def find_orphaned_releases(
    deployments_by_env: Dict[str, List[Dict]],
    helm_releases_by_kubeconfig: Dict[str, Dict[str, List[Dict]]]
) -> Dict[str, Dict[str, List[Dict]]]:
    im_deployments_by_cluster = defaultdict(set)
    for deployments in deployments_by_env.values():
        for deployment in deployments:
            im_deployments_by_cluster[deployment["cluster"]].add((deployment["namespace"], deployment["name"]))

    helm_releases_by_cluster = defaultdict(set)
    kubeconfig_to_cluster = {}
    for kubeconfig_name, namespaces in helm_releases_by_kubeconfig.items():
        cluster = normalize_cluster_name(kubeconfig_name)
        kubeconfig_to_cluster[kubeconfig_name] = cluster

        for namespace, releases in namespaces.items():
            for release in releases:
                helm_releases_by_cluster[cluster].add((namespace, release["base_name"]))

    orphaned_by_kubeconfig = {}
    for kubeconfig_name, namespaces in helm_releases_by_kubeconfig.items():
        cluster = kubeconfig_to_cluster[kubeconfig_name]
        orphaned_keys = helm_releases_by_cluster[cluster] - im_deployments_by_cluster.get(cluster, set())

        if orphaned_keys:
            orphaned_by_kubeconfig[kubeconfig_name] = {}
            for namespace, releases in namespaces.items():
                orphaned_releases = [release for release in releases if (namespace, release["base_name"]) in orphaned_keys]
                if orphaned_releases:
                    orphaned_by_kubeconfig[kubeconfig_name][namespace] = orphaned_releases

    return orphaned_by_kubeconfig


def print_orphaned_releases(orphaned_by_kubeconfig: Dict[str, Dict[str, List[Dict]]]):
    total = sum(
        len({r["base_name"] for r in releases})
        for namespaces in orphaned_by_kubeconfig.values()
        for releases in namespaces.values()
    )

    print_separator("Comparison")
    print(f"\nOrphaned Deployments only in Helm ({total}):")

    for kubeconfig_name in sorted(orphaned_by_kubeconfig.keys()):
        namespaces = orphaned_by_kubeconfig[kubeconfig_name]
        for namespace in sorted(namespaces.keys()):
            releases = namespaces[namespace]
            base_names = sorted({release["base_name"] for release in releases})
            print(f"  {kubeconfig_name} / {namespace} ({len(base_names)}):")
            for base_name in base_names:
                print(f"    - {base_name}")


def generate_helm_commands(orphaned_by_kubeconfig: Dict[str, Dict[str, List[Dict]]],kubeconfigs: List[str]) -> List[str]:
    commands = []
    kubeconfig_map = {os.path.basename(kubeconfig): kubeconfig for kubeconfig in kubeconfigs}

    for kubeconfig_name, namespaces in orphaned_by_kubeconfig.items():
        kubeconfig_path = kubeconfig_map.get(kubeconfig_name)
        for _, releases in namespaces.items():
            for release in releases:
                cmd = f"helm uninstall {release['name']} --namespace {release['namespace']}"
                if kubeconfig_path:
                    cmd += f" --kubeconfig {kubeconfig_path}"
                commands.append(cmd)

    return sorted(commands)


def main():
    parser = argparse.ArgumentParser()
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
