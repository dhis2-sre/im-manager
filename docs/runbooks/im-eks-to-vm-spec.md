# Spec: migrate the IM control plane from EKS to a Hetzner VM (docker compose)

Migrate the Instance Manager control plane off the AWS EKS `instance-cluster` service (DEVOPS-684,
EKS in extended support) onto **two decoupled Hetzner Cloud VMs** running docker compose — prod on its
own VM, dev + feature environments on a second VM. Instance workloads already run on the Hetzner
`k3s-prod` cluster and are not touched. Cutover mechanics (dump/restore, maintenance window, rollback)
are in [`im-eks-to-vm-runbook.md`](./im-eks-to-vm-runbook.md); this doc is the target architecture and
the per-component disposition.

## Architecture

**Two decoupled Hetzner VMs** running docker compose:

- **prod VM** — the `prod` stack only.
- **dev+feat VM** — the `dev` stack plus ephemeral per-PR `feat-*` stacks.

**Per-env isolated stacks:** each env is its own compose project with its own IM + Postgres +
RabbitMQ + Redis and volumes. Splitting prod onto its own VM isolates the public, credential-bearing
prod env from the churny, less-trusted feature envs (arbitrary PR code, created/destroyed constantly),
and rides the existing prod/nonprod credential boundary. On each VM the only shared singletons are
**Traefik (ingress) and an observability shipper**.

The IM control plane is kept off the k3s workload cluster deliberately (failure isolation, stable
resources, independent upgrade lifecycle, tighter security boundary). AWS is retained for S3 (DB
blobs), KMS (SOPS), and SES (mail).

## VMs

Both x86 (the image is amd64-only), Ubuntu 24.04, in **nbg1** (same region as k3s-prod). Public
IPv4/IPv6; **not** joined to the k3s private network (decoupling + blast radius). hcloud firewall:
in 22 (restrict to admin / CI ranges), 80, 443; out all. Postgres/RabbitMQ/Redis bound to the compose
network only. Non-root `im` user, SSH keys only, disk encryption. Docker Engine + compose plugin (the
IM image already bundles helm/helmfile/kubectl/pg_dump).

| VM | Type | Holds | Sizing basis |
|---|---|---|---|
| **prod** | `cpx22` (2 vCPU / 4 GB / 80 GB) | prod stack + Traefik + shipper | 1 stack ~1 GB; RAM comfortable. Watch disk (80 GB) only for very large instance DB dumps — attach a Hetzner Volume if needed |
| **dev+feat** | `cpx32` (4 vCPU / 8 GB / 160 GB) | dev + up to 5 feat stacks + Traefik + shipper | dev + 2-3 feat (the norm) ~4.5-6 GB, comfortable; the rare 5-env peak (~7 GB) runs with acceptable degradation |

Resizing up is a reboot on Hetzner (scale in place, no rebuild), so start here and bump the dev+feat
VM to `cpx42` (16 GB) only if real feature load makes it tight.

## Software layout

**Shared singletons — one set per VM** (each VM runs its own):
- **Traefik** — Docker provider; routes by container labels; TLS via Let's Encrypt (see DNS/TLS).
- **Observability shipper** — log/metric/trace forwarding to the existing IM monitoring stack (see
  Monitoring & telemetry). No new Grafana on either VM.

**Per-env compose project** (`docker compose -p <env>`): IM, Postgres, RabbitMQ, Redis; own volumes;
`mem_limit` per service. `prod` runs on the prod VM; `dev` and every `feat-*` run on the dev+feat VM.
No per-env Jaeger (traces go to the shared shipper).

## Target ("default") cluster

IM creates a `default` cluster row at boot (`cmd/serve/main.go:732`) with **nil** kubeconfig. A nil
kubeconfig means "use my own in-cluster identity" (`pkg/instance/kubernetes.go:158-164`,
`BuildConfigFromFlags("","")`). **On a VM there is no in-cluster identity, so nil is a footgun** — any
group that resolves to a nil-config cluster breaks at deploy time.

Decision: **do not leave it nil.** Set the `default` cluster's kubeconfig to the **k3s-prod**
(SOPS-encrypted) kubeconfig, so `default` resolves to the real workload cluster. k3s-prod's API
endpoint is already reachable over its public IP (IM reaches it that way from EKS today).

Requirements:
- Every group must resolve to a cluster with a stored kubeconfig — either explicitly bound
  (`ClusterID`) or via the now-k3s-prod `default`. Verify with the Gate A query in the runbook (no
  group may resolve to a nil-config cluster).
- Do **not** set `GROUP_NAMES` / `GROUP_NAMESPACES` / `GROUP_HOSTNAMES` on the VM in a way that
  recreates groups on the nil default (`cmd/serve/main.go:740-777`).
- Optional hardening (code, later): make IM warn/refuse to start if a cluster has a nil kubeconfig
  while not running in-cluster, so this cannot silently regress.

## Migrate / drop / replace: `instance-cluster` components

| Component | Action | Target on VM |
|---|---|---|
| EKS control plane + node groups + `aws-auth` | DROP | the VM |
| cluster-autoscaler + ASG tags | DROP | none — size for peak |
| VPC / subnets / NAT / security groups | DROP | hcloud firewall |
| ingress-nginx (Service→ELB) | REPLACE | Traefik (per VM, Docker provider) |
| cert-manager + `cert-issuer-prod` (LE) | REPLACE | Traefik + Let's Encrypt |
| Route53 records | MIGRATE | repoint per env to the prod / dev+feat VM IP; zone stays |
| EBS CSI + StorageClass | DROP | docker named volumes |
| Postgres / RabbitMQ / Redis (in-cluster helm, per env) | MIGRATE | per-env compose services + volumes |
| IRSA `instance_manager_role` (KMS + S3) | REPLACE | scoped static IAM key per VM (prod / nonprod) |
| KMS keys `im-{prod,nonprod}-secrets`, `im-helm-*` | KEEP (AWS) | static-key `kms:Decrypt` |
| S3 `im-databases-{dev,feature,prod}` | KEEP (AWS) | unchanged, shared by all envs |
| SES / SMTP (`services/im`, `modules/smtp`) | KEEP (AWS) | unchanged (already separate) |
| Velero → S3 + `im-backup` schedules | REPLACE | cron `pg_dump`→S3 (see Backups) |
| kube-prometheus-stack / Loki / promtail (`grafana.im`) | DROP (stale on EKS) | ship to existing k3s IM monitoring |
| EKS control-plane → CloudWatch audit logs | DROP | none (no k8s API server) |
| KEDA / metrics-server | N/A | workload concern, lives on k3s-prod |
| pre-created namespaces, local kubeconfig, `whoami` | DROP | vestigial |

## AWS access (replaces IRSA)

**Two scoped IAM users, one per VM** (precedent: `instance-manager-ci`), so prod credentials never sit
on the box that runs arbitrary PR code. This rides the existing prod/nonprod KMS + S3 split:

- **prod VM key** — S3 on `im-databases-prod`; `kms:Decrypt`/`Encrypt` on `im-prod-secrets` +
  `im-helm-prod-secrets`; `ses:SendRawEmail`.
- **dev+feat VM key** — S3 on `im-databases-{dev,feature}`; KMS on `im-nonprod-secrets` +
  `im-helm-nonprod-secrets`; `ses:SendRawEmail`.

A compromised feature env therefore cannot reach prod's KMS keys, prod's S3, or prod's cluster
kubeconfigs. Leave `S3_ENDPOINT` unset so the SDK targets real S3. Confirm VM egress to S3
(eu-west-1), KMS (eu-central-1), and SES.

## DNS + TLS

Each VM owns its hostnames and its own Traefik + Let's Encrypt certs.

- **prod VM:** `im`, `api.im`. TLS via HTTP-01.
- **dev+feat VM:** `im-dev`, `api.im-dev`, `im-feat`, `*.im-feat`. TLS via HTTP-01 for the fixed hosts
  and **DNS-01 via Route53** for the `*.im-feat` wildcard (uses the dev+feat VM's nonprod AWS creds —
  one wildcard cert instead of one per PR).
- Leave `grafana.im` on k3s. Lower TTLs (~60s) before cutover. Route53 zone stays.

## Backups — not Velero

Velero is dropped: it is k8s-native (backs up namespace manifests + PV snapshots); on a VM there are
no k8s objects, and compose files live in git. Replace with:

- **`pg_dump` (custom format) → S3**, per env, via a cron/backup container on each VM using that VM's
  IAM key.
- **Prod** (prod VM): daily + weekly. Retention 7 d / 30 d (mirrors the Velero daily-168 h /
  weekly-720 h TTLs), enforced via S3 lifecycle rules.
- **Dev** (dev+feat VM): daily. Feature envs are not backed up.
- **VM/volume snapshots** (hcloud) as a coarse whole-box safety net.
- Not backed up: Redis, RabbitMQ (transient). Instance data itself already lives durably in
  `im-databases` S3.
- Bucket: reuse the existing `instance-cluster-*-backup` bucket or a new `im-backup` bucket.

## Monitoring & telemetry wiring

The IM Grafana already runs on k3s-prod, independent of EKS — nothing to rebuild. What IM emits and
how the VM feeds that stack:

- **Logs:** IM writes JSON to **stdout** (`cmd/serve/main.go:113`). On the VM: forward container
  stdout via the Docker Loki logging driver or a Grafana Alloy/promtail agent → the IM Loki.
- **Traces:** IM exports OpenTelemetry spans to a **Jaeger collector** (`JAEGER_HOST`/`JAEGER_PORT`,
  `main.go:874-902`). On the VM: point every env at one **shared collector** (a single
  Jaeger/OTel-Collector container, or the existing stack's collector) instead of a per-env Jaeger.
- **Metrics:** IM exposes **no Prometheus `/metrics` endpoint**. Host/container metrics come from a
  node-exporter + cAdvisor on the VM, scraped by (or pushed to) the existing Prometheus.

To confirm before build:
- Whether the k3s IM monitoring stack exposes Loki-push + a trace collector endpoint **reachable from
  the VM** (if not, run a minimal collector on the VM that forwards on).
- IM uses the now-deprecated OTel Jaeger exporter — consider switching to OTLP for future-proofing
  (not required for the migration).

## Feature-env CI

Today per-PR feature envs are created by a GitHub Actions reusable workflow running `skaffold` into a
k8s namespace. On the VM there is no k8s for that.

Recommendation: **SSH from GitHub Actions** running a small `im-envctl` lifecycle script on the
**dev+feat VM** (CI never touches the prod host).
- PR opened/updated → build image (existing) → SSH → `im-envctl up <pr> <image-tag>`: render
  `envs/feat-<pr>.env`, `docker compose -p feat-<pr> up -d` with Traefik labels for
  `pr-<pr>.im-feat.dhis2.org`.
- PR closed → SSH → `im-envctl down <pr>`: `docker compose -p feat-<pr> down -v` + cleanup.
- No per-PR DNS/TLS work: the `*.im-feat` wildcard record + wildcard cert cover it; Traefik auto-routes
  by label.
- Security: a **dedicated deploy key restricted by a forced command** (authorized_keys
  `command="..."`) so it can only run `im-envctl`, not open a shell; restrict SSH ingress to the CI
  runner ranges (or tunnel via WireGuard/Tailscale).

Rejected: a bespoke deploy-agent HTTP hook on the VM (more to build and secure, no real benefit at
1-5 feature envs); keeping feature envs on k3s (reintroduces the k8s coupling we are removing).

## Other requirements

- **Docker Hub creds** (`DOCKER_HUB_USERNAME`/`PASSWORD`) set on both VMs to avoid image-pull rate
  limits (a documented manual pain point on EKS).
- **Secret continuity:** carry `INSTANCE_PARAMETER_ENCRYPTION_KEY`, the SOPS `KMS_ARN`, `PRIVATE_KEY`,
  and `REFRESH_TOKEN_SECRET_KEY` byte-for-byte from the EKS config, or existing encrypted data /
  tokens will not decrypt.

## Open items

- Confirm the k3s IM monitoring stack's ingestion endpoints are reachable from the VM.
- Confirm the k3s-prod kubeconfig to seed the `default` cluster (and that all groups resolve to a
  stored kubeconfig — Gate A).
- Provision/scope the two VM IAM users (prod / nonprod) and the S3 backup bucket + lifecycle rules.

## References

- Cutover procedure, gates, rollback: [`im-eks-to-vm-runbook.md`](./im-eks-to-vm-runbook.md)
