#!/usr/bin/env python3

"""
Find Expiring Certificates

This script checks the TLS certificates Ingresses expect cert-manager to provide and
reports the ones that are unhealthy: expiring soon, already expired, never issued, or
no longer being renewed.

cert-manager renews 30 days before expiry, so anything inside the default 14 day
threshold means renewal has been failing for two weeks. A served certificate with no
Certificate resource behind it is reported regardless of expiry, because nothing is
renewing it and it will expire silently. Causes seen in practice: the Certificate being
garbage collected when the Ingress that owned it was deleted, stuck HTTP-01 challenges,
and Let's Encrypt rate limits.

The script:
1. Collects the TLS secrets Ingresses reference, and the issuer they ask for
2. Compares them against the Secrets and Certificates that actually exist
3. Reports anything expiring within the threshold or otherwise broken
"""

import argparse
import base64
import json
import os
import subprocess
import sys
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import Dict, List, Optional, Set, Tuple


CERT_MANAGER_SECRET_ANNOTATION = "cert-manager.io/certificate-name"
INGRESS_ISSUER_ANNOTATIONS = ("cert-manager.io/cluster-issuer", "cert-manager.io/issuer")
DEFAULT_THRESHOLD_DAYS = 14
DEFAULT_GRACE_MINUTES = 60

STATE_EXPIRING = "expiring"
STATE_EXPIRED = "expired"
STATE_NOT_RENEWING = "not renewing"
STATE_NEVER_ISSUED = "never issued"
STATE_UNREADABLE = "unreadable"


@dataclass
class IngressReference:
    namespace: str
    secret_name: str
    hosts: List[str] = field(default_factory=list)
    ingresses: List[str] = field(default_factory=list)
    oldest_ingress: Optional[datetime] = None
    requests_certificate: bool = False
    days_left: Optional[int] = None


@dataclass
class Finding:
    reference: IngressReference
    state: str
    days_left: Optional[int] = None

    @property
    def sort_key(self) -> int:
        if self.days_left is not None:
            return self.days_left
        return -9999

    def describe(self) -> str:
        if self.state == STATE_EXPIRED:
            return f"EXPIRED {abs(self.days_left)} days ago"
        if self.state == STATE_EXPIRING:
            return f"expires in {self.days_left} days"
        if self.state == STATE_NOT_RENEWING:
            expiry = f", expires in {self.days_left} days" if self.days_left is not None else ""
            return f"Certificate resource is missing, nothing is renewing it{expiry}"
        if self.state == STATE_NEVER_ISSUED:
            return "Secret does not exist, the certificate was never issued"
        return "certificate could not be read"


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


def parse_timestamp(value: str) -> Optional[datetime]:
    try:
        return datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    except (TypeError, ValueError):
        return None


def get_ingress_references(kubeconfig: str) -> Dict[Tuple[str, str], IngressReference]:
    ingresses = kubectl_json(kubeconfig, "get", "ingress", "--all-namespaces")

    references: Dict[Tuple[str, str], IngressReference] = {}
    for ingress in ingresses["items"]:
        metadata = ingress["metadata"]
        namespace = metadata["namespace"]
        name = metadata["name"]
        annotations = metadata.get("annotations") or {}
        created = parse_timestamp(metadata.get("creationTimestamp"))
        requests_certificate = any(annotation in annotations for annotation in INGRESS_ISSUER_ANNOTATIONS)

        for tls in ingress["spec"].get("tls", []):
            secret_name = tls.get("secretName")
            if not secret_name:
                continue

            reference = references.setdefault(
                (namespace, secret_name), IngressReference(namespace=namespace, secret_name=secret_name)
            )
            reference.ingresses.append(name)
            reference.requests_certificate = reference.requests_certificate or requests_certificate
            if created and (reference.oldest_ingress is None or created < reference.oldest_ingress):
                reference.oldest_ingress = created

            for host in tls.get("hosts", []):
                if host not in reference.hosts:
                    reference.hosts.append(host)

    for reference in references.values():
        reference.hosts.sort()
        reference.ingresses.sort()

    return references


def get_tls_secrets(kubeconfig: str) -> Dict[Tuple[str, str], dict]:
    secrets = kubectl_json(kubeconfig, "get", "secret", "--all-namespaces", "--field-selector", "type=kubernetes.io/tls")
    return {(s["metadata"]["namespace"], s["metadata"]["name"]): s for s in secrets["items"]}


def get_existing_certificates(kubeconfig: str) -> Set[Tuple[str, str]]:
    certificates = kubectl_json(kubeconfig, "get", "certificate", "--all-namespaces")
    return {(c["metadata"]["namespace"], c["metadata"]["name"]) for c in certificates["items"]}


def read_not_after(pem: bytes) -> Optional[datetime]:
    result = subprocess.run(["openssl", "x509", "-noout", "-enddate"], input=pem, capture_output=True)
    if result.returncode != 0:
        return None

    end_date = result.stdout.decode().strip().removeprefix("notAfter=")
    try:
        return datetime.strptime(end_date, "%b %d %H:%M:%S %Y %Z").replace(tzinfo=timezone.utc)
    except ValueError:
        return None


def check_cluster(
    kubeconfig: str,
    threshold_days: int,
    grace_minutes: int,
    ignored_namespaces: Set[str]
) -> Tuple[List[IngressReference], List[Finding]]:
    print(f"  Checking kubeconfig: {os.path.basename(kubeconfig)}")

    references = get_ingress_references(kubeconfig)
    secrets = get_tls_secrets(kubeconfig)
    certificates = get_existing_certificates(kubeconfig)

    now = datetime.now(timezone.utc)
    grace = timedelta(minutes=grace_minutes)

    checked: List[IngressReference] = []
    findings: List[Finding] = []

    for key, reference in sorted(references.items()):
        if reference.namespace in ignored_namespaces:
            continue

        secret = secrets.get(key)

        if secret is None:
            recently_created = reference.oldest_ingress is not None and now - reference.oldest_ingress < grace
            if reference.requests_certificate and not recently_created:
                findings.append(Finding(reference=reference, state=STATE_NEVER_ISSUED))
            continue

        annotations = secret["metadata"].get("annotations") or {}
        certificate_name = annotations.get(CERT_MANAGER_SECRET_ANNOTATION)
        if certificate_name is None:
            continue

        checked.append(reference)

        pem = (secret.get("data") or {}).get("tls.crt")
        not_after = read_not_after(base64.b64decode(pem)) if pem else None
        if not_after is None:
            findings.append(Finding(reference=reference, state=STATE_UNREADABLE))
            continue

        days_left = int((not_after - now).total_seconds() / 86400)
        reference.days_left = days_left

        if (reference.namespace, certificate_name) not in certificates:
            findings.append(Finding(reference=reference, state=STATE_NOT_RENEWING, days_left=days_left))
        elif days_left < 0:
            findings.append(Finding(reference=reference, state=STATE_EXPIRED, days_left=days_left))
        elif days_left < threshold_days:
            findings.append(Finding(reference=reference, state=STATE_EXPIRING, days_left=days_left))

    return checked, sorted(findings, key=lambda f: f.sort_key)


def print_checked_certificates(checked_by_kubeconfig: Dict[str, List[IngressReference]]):
    print_separator("Certificates Served by Ingresses")

    for kubeconfig_name in sorted(checked_by_kubeconfig.keys()):
        checked = checked_by_kubeconfig[kubeconfig_name]
        print(f"\n{kubeconfig_name} ({len(checked)} certificates):")

        for reference in checked:
            expiry = f"expires in {reference.days_left}d" if reference.days_left is not None else "expiry unknown"
            print(f"  {reference.namespace} / {reference.secret_name} ({', '.join(reference.hosts)}) {expiry}")


def print_findings(findings_by_kubeconfig: Dict[str, List[Finding]], threshold_days: int):
    total = sum(len(findings) for findings in findings_by_kubeconfig.values())

    print_separator("Comparison")
    print(f"\nUnhealthy certificates, expiring within {threshold_days} days or otherwise broken ({total}):")

    for kubeconfig_name in sorted(findings_by_kubeconfig.keys()):
        for finding in findings_by_kubeconfig[kubeconfig_name]:
            reference = finding.reference
            print(f"  {kubeconfig_name} / {reference.namespace} / {', '.join(reference.hosts)}: {finding.describe()}")
            print(f"    secret: {reference.secret_name}")
            print(f"    served by: {', '.join(reference.ingresses)}")


def generate_remediation_commands(findings_by_kubeconfig: Dict[str, List[Finding]], kubeconfigs: List[str]) -> List[str]:
    commands = []
    kubeconfig_map = {os.path.basename(kubeconfig): kubeconfig for kubeconfig in kubeconfigs}

    for kubeconfig_name, findings in findings_by_kubeconfig.items():
        kubeconfig_path = kubeconfig_map.get(kubeconfig_name)

        for finding in findings:
            reference = finding.reference
            if finding.state == STATE_NOT_RENEWING:
                cmd = (
                    f"kubectl --namespace {reference.namespace} annotate ingress {reference.ingresses[0]}"
                    f" im.dhis2.org/cert-nudge=$(date +%F) --overwrite"
                )
            else:
                cmd = f"kubectl --namespace {reference.namespace} describe certificate {reference.secret_name}"

            if kubeconfig_path:
                cmd += f" --kubeconfig {kubeconfig_path}"
            commands.append(cmd)

    return commands


def main():
    parser = argparse.ArgumentParser(
        description="Find TLS certificates served by Ingresses that are expiring, expired or no longer being renewed."
    )
    parser.add_argument("--kubeconfig", action="append", help="Path to kubeconfig file (can be used multiple times)")
    parser.add_argument("--days", type=int, default=DEFAULT_THRESHOLD_DAYS, help=f"Expiry threshold in days (default: {DEFAULT_THRESHOLD_DAYS})")
    parser.add_argument(
        "--grace-minutes",
        type=int,
        default=DEFAULT_GRACE_MINUTES,
        help=f"Ignore missing Secrets for Ingresses younger than this (default: {DEFAULT_GRACE_MINUTES})"
    )
    parser.add_argument("--ignore-namespace", action="append", help="Namespace to skip (can be used multiple times)")
    args = parser.parse_args()

    kubeconfigs = args.kubeconfig or []
    if not kubeconfigs:
        print("Error: At least one --kubeconfig must be provided")
        sys.exit(1)

    ignored_namespaces = set(args.ignore_namespace or [])

    print("Expiring TLS Certificates Check")
    print_separator()
    print(f"Using {len(kubeconfigs)} kubeconfig(s): {', '.join(kubeconfigs)}")
    if ignored_namespaces:
        print(f"Ignoring namespace(s): {', '.join(sorted(ignored_namespaces))}")
    print()

    print("Fetching Ingresses, TLS Secrets and Certificates from Kubernetes...")
    checked_by_kubeconfig = {}
    findings_by_kubeconfig = {}

    for kubeconfig in kubeconfigs:
        checked, findings = check_cluster(kubeconfig, args.days, args.grace_minutes, ignored_namespaces)
        kubeconfig_name = os.path.basename(kubeconfig)
        checked_by_kubeconfig[kubeconfig_name] = checked
        if findings:
            findings_by_kubeconfig[kubeconfig_name] = findings

    total = sum(len(checked) for checked in checked_by_kubeconfig.values())
    print(f"Found {total} certificates served by Ingresses")

    print_checked_certificates(checked_by_kubeconfig)
    print_findings(findings_by_kubeconfig, args.days)

    commands = generate_remediation_commands(findings_by_kubeconfig, kubeconfigs)
    if commands:
        print(f"\n\nCommands to investigate unhealthy certificates ({len(commands)}):")
        for cmd in commands:
            print(cmd)


if __name__ == "__main__":
    main()
