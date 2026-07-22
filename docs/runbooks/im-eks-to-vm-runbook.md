# Runbook: Migrate the IM control plane from EKS to a Hetzner VM

Relocate the Instance Manager (IM) control plane off the AWS EKS cluster (`DEVOPS-684`, EKS in
extended support) onto **two Hetzner VMs** running docker compose — prod on its own VM, dev + feature
environments on a second VM — then decommission EKS. See
[`im-eks-to-vm-spec.md`](./im-eks-to-vm-spec.md) for the target architecture; this runbook is the
cutover procedure.

Approach: **rehearsed `pg_dump` / `pg_restore` in a short maintenance window.** No streaming
replication, no master/slave. Instance workloads already run on Hetzner and are not touched.

**Two-VM target:** `prod` migrates to the **prod VM** (`cpx22`); `dev` migrates to the **dev+feat VM**
(`cpx32`); per-PR `feat-*` envs are created fresh by CI and are **not** migrated. The steps below
describe the **prod cutover** to the prod VM — run the same procedure for `dev` against the dev+feat VM
(its own DB dump/restore and its own `im-dev` / `api.im-dev` hostnames). Prod and dev cut over
independently.

## Goals and success criteria

- IM API/UI served from the VM at the existing hostnames (`api.im.dhis2.org`, `im.dhis2.org`).
- Zero interruption to running DHIS2 instances (they live on Hetzner; IM being down only pauses
  management operations).
- Control-plane downtime limited to a single-digit-minute maintenance window.
- EKS retired afterwards.

## Scope and non-goals

In scope: the IM service and its stateful dependencies (Postgres metadata DB, Redis/Valkey,
RabbitMQ, Jaeger), TLS termination, and the AWS auth swap.

Out of scope for this migration:

- Migrating instance workloads (already on Hetzner).
- Moving DB blobs out of S3. The `im-databases-prod` bucket stays in AWS; both old and new IM point
  at the **same bucket**, so there is no blob migration and no cutover risk there.
- Changing the SOPS backend. Keep AWS KMS (see gate C). Switching to `age` would require
  re-encrypting every stored kubeconfig and stack-parameter file and is a separate decision.
- High availability. IM is single-replica today; the VM is a single host. Acceptable given a fast
  rebuild path, but noted.

## What state moves, and what does not

| Component | Action | Rationale |
|---|---|---|
| Postgres (IM metadata) | **Migrate** via dump/restore | Users, groups, clusters (encrypted kubeconfigs), deployments, instances, token state. Small (metadata only). |
| Redis / Valkey | **Start fresh** | Holds refresh-token state only. Loss = users re-login. Not worth migrating. |
| RabbitMQ | **Start fresh** | Transient events/notifications. A brief gap is fine. |
| Jaeger / tracing | **Not per-env** | Traces go to the shared observability shipper, not a per-env Jaeger. |
| S3 (`im-databases-prod`) | **Keep in AWS, shared** | Instance DB dumps/blobs. No migration. |
| KMS (SOPS) | **Keep in AWS** | Unwraps SOPS data keys for the kubeconfigs stored in Postgres. |
| SES (SMTP) | **Keep in AWS** | Password reset / invite mail. |

## Critical correctness gates (verify BEFORE scheduling the window)

These are the things that fail silently. Resolve all three during the rehearsal, not during cutover.

### Gate A - no group may resolve to the in-cluster "default" cluster

On EKS, a group whose cluster has no stored kubeconfig is managed using IM's own in-cluster pod
identity (`pkg/instance/kubernetes.go:158-164`, `newRestConfig` falls back to
`clientcmd.BuildConfigFromFlags("","")`). **On a VM there is no in-cluster config, so any such group
breaks at deploy time.** Every group must point at a registered cluster that has an encrypted
kubeconfig (the Hetzner clusters).

Check against the live DB (verify table/column names against the current schema):

```sql
-- Groups that would fall back to in-cluster config (must return zero rows before cutover):
SELECT g.name, g.cluster_id
FROM groups g
LEFT JOIN clusters c ON c.id = g.cluster_id
WHERE g.cluster_id IS NULL
   OR c.configuration IS NULL;   -- NULL configuration == the in-cluster "default" cluster

-- Inventory of clusters and whether each has a stored kubeconfig:
SELECT id, name, (configuration IS NOT NULL) AS has_kubeconfig FROM clusters;
```

If any rows come back from the first query, bind those groups to their Hetzner cluster (or set an
explicit kubeconfig on the cluster they use) before proceeding. Also note the repo config is stale
relative to reality: `helm/data/values/prod/values.yaml` still seeds `GROUP_NAMES=play,qa,...` and
`skaffold.yaml` still deploys `im-group` namespaces onto EKS. On the VM, do **not** set the
`GROUP_NAMES` / `GROUP_NAMESPACES` / `GROUP_HOSTNAMES` env vars in a way that recreates groups
attached to the nil default cluster (`cmd/serve/main.go:740-777`). The `default` cluster row itself
(`cmd/serve/main.go:732`) becomes meaningless once IM is off EKS; leave it only if no group uses it.

### Gate B - secret continuity

Several secrets must be carried over **byte-for-byte**, or existing data becomes undecryptable:

| Secret | If it changes |
|---|---|
| `INSTANCE_PARAMETER_ENCRYPTION_KEY` | Existing instance parameters (AES-encrypted) cannot be decrypted. **Must match exactly.** |
| SOPS key (`SOPS_KMS_ARN`) | Stored cluster kubeconfigs cannot be decrypted. **Must match exactly** (same KMS key). |
| `PRIVATE_KEY` (RS256) | Signs access tokens. Change invalidates in-flight tokens; tokens are short-lived so low impact, but carry it over for continuity. |
| `REFRESH_TOKEN_SECRET_KEY` (HS256) | Change invalidates all refresh tokens -> every user must log in again. Carry over unless a forced re-login is acceptable. |
| `SESSION_SECRET` | Signs the short-lived OAuth state cookie. Transient; low impact. |

### Gate C - AWS access without IRSA

On EKS, S3 + KMS + SES auth comes from IRSA (the pod's IAM role). The VM has no IRSA, so provision
**one scoped IAM user access key** and set it on the VM. The AWS SDK default credential chain and
the existing helmfile env forwarding (`pkg/instance/helmfile.go:143-148`) already support static
keys, so no code change is needed.

The IAM user needs, at minimum:

- S3 read/write on `im-databases-prod`.
- `kms:Decrypt` (and `kms:Encrypt` for creating/updating clusters) on the SOPS KMS key.
- SES send permissions.

Set on the VM: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` / `AWS_DEFAULT_REGION`.
Leave `S3_ENDPOINT` **unset** so the SDK talks to real AWS S3 (the local-dev value points at MinIO).
Confirm the VM has network egress to S3, KMS, and SES endpoints.

## Phase 0 - Provision and pre-stage the VM (days ahead)

1. **VMs.** Provision both (nbg1, Ubuntu 24.04, x86): the **prod VM** `cpx22` (2 vCPU / 4 GB / 80 GB)
   for the single prod stack, and the **dev+feat VM** `cpx32` (4 vCPU / 8 GB / 160 GB) for dev + up to
   5 feature stacks. RAM is comfortable at these sizes; the watch-item is prod disk (80 GB) if an
   instance has a very large DB dump streamed through scratch (`dataSizeLimit` was `30Gi` on EKS) —
   attach a Hetzner Volume if needed. Resizing up is a reboot (no rebuild), so bump the dev+feat VM to
   `cpx42` only if real feature load makes it tight.
2. **Install** Docker Engine + Compose plugin.
3. **Firewall / network hardening.** This VM will hold the Postgres DB containing the
   SOPS-decryptable kubeconfigs for **every managed cluster** plus all user/token state - it is
   effectively the master key to the whole fleet. Treat it as a high-value target:
   - Public ingress: only `80` and `443` (Traefik) and SSH (ideally restricted / behind VPN).
   - Bind Postgres, Redis, RabbitMQ to `127.0.0.1` or a private network only - never publicly. (The
     compose file already binds the RabbitMQ management port to `127.0.0.1`; do the same for the
     data ports.)
   - Enable disk encryption; harden SSH (keys only, no root password).
4. **Pull the prod image** and prepare the compose stack (`prod` profile of `docker-compose.yml`).
   Configure real AWS S3 (Gate C), same Hetzner clusters, same secrets (Gate B).
5. **TLS.** Configure the outer Traefik + Let's Encrypt HTTP-01 as documented in `README.md`
   ("Let's Encrypt on the outer Traefik"). During rehearsal use LE **staging** to avoid rate limits;
   switch to LE production before the real cutover.

## Phase 1 - Rehearsal / dry run (days ahead, repeatable)

This validates the entire path so the real window is just a re-sync. Run it against a **copy** of the
prod DB; it touches nothing in prod.

1. Dump the prod DB (see Appendix A) and restore into the VM Postgres (Appendix B).
2. Bring up the IM compose stack on the VM. Reach it via a temporary hostname or an
   `/etc/hosts` override so DNS is untouched.
3. Smoke test (Appendix C): log in, list instances, deploy **and destroy a throwaway instance on
   Hetzner**, trigger a DB backup/restore (exercises S3 + KMS + pod-exec against Hetzner), check
   logs and that the inspector is running.
4. Confirm all three gates pass in practice: no group errored on the nil default cluster (A), data
   decrypts (B), S3/KMS/SES reachable with the static key (C).
5. Tear down the throwaway test data. Record how long the dump + restore + smoke test actually took;
   that is your real window estimate.

Repeat until a rehearsal is clean end-to-end.

## Phase 2 - Pre-cutover (day before)

1. **Lower DNS TTL** on `api.im.dhis2.org` and `im.dhis2.org` to ~60s. High TTL is the most common
   hidden cause of a long cutover.
2. Switch Traefik on the VM from LE staging to LE production and confirm a valid cert is issued.
3. Announce the maintenance window to users.
4. **Freeze deploys.** Ensure no instance deploys are scheduled during the window (see the in-flight
   caveat below). Optionally coordinate a "no new deploys" period.

## Phase 3 - Cutover window

Ordered steps. Keep EKS intact throughout - it is the rollback.

1. **Confirm no deploy is in progress.** A deploying instance's seed pod curls IM with
   `IM_ACCESS_TOKEN` and only writes its success marker on HTTP 200 (see `CLAUDE.md`, seed pattern).
   If IM moves mid-deploy, the seed fails, no marker is written, and the pod can enter
   `CrashLoopBackOff` retrying with an expired token. Wait for any in-flight deploy to finish.
2. **Quiesce writes on EKS.** Scale the IM deployment to zero:
   `kubectl -n instance-manager-prod scale deploy/im-manager-prod --replicas=0`
   (confirm the exact deployment name). This stops all writes to the EKS DB.
3. **Final dump** of the EKS DB (Appendix A).
4. **Restore** into the VM Postgres, clean (Appendix B).
5. **Start IM on the VM** (`docker compose --profile prod up -d`).
6. **Smoke test on the VM before flipping DNS**, via the temp hostname / direct IP (Appendix C,
   abbreviated: login + list instances + one deploy/destroy).
7. **Flip DNS** for both hostnames to the VM's public IP.
8. **Verify via the real hostnames** once DNS propagates (TTL was lowered): HTTPS cert valid, login,
   list, deploy/destroy a throwaway.

Expected user-visible downtime: the dump + restore + smoke-test window (single-digit minutes) plus
DNS propagation. Running DHIS2 instances are unaffected the whole time.

## Phase 4 - Post-cutover validation

- Log in via UI and API.
- List instances; confirm data matches EKS (counts, names).
- Deploy a throwaway instance on Hetzner, confirm it becomes healthy, then destroy it. This exercises
  the deploy-time access-token refresh, helmfile against a remote cluster, and the seed callback.
- Trigger a DB blob upload + download (S3 path) and a backup/restore (S3 + KMS + pod-exec).
- Confirm the inspector (TTL reaper) is running and events/notifications flow.
- Watch logs for `Environment variable not found` warnings (`injectEnv`) - a missing AWS/KMS var
  shows up here.

## Rollback

EKS is untouched and its DB holds all state up to the moment IM was scaled to zero. Rollback:

1. Flip DNS back to the EKS ingress.
2. Scale IM on EKS back up: `kubectl -n instance-manager-prod scale deploy/im-manager-prod --replicas=1`.

Caveat: any writes made against the VM after cutover (new users, instances, deploys) are **not** in
the EKS DB and are lost on rollback. Define a rollback cutoff - e.g. "if a blocking issue appears
within the first N minutes / before the first real deploy, roll back; after that, fix forward." Keep
the final EKS dump file as a safety artifact regardless.

## Phase 5 - Decommission EKS (after a stability soak, e.g. 3-7 days)

Only after the VM has run cleanly and a real deploy/destroy has succeeded through it:

1. Remove the IM releases from `skaffold.yaml` (IM chart, Postgres/Redis/RabbitMQ/Jaeger releases,
   `im-group` namespaces) and the stale `GROUP_*` prod config.
2. Delete the EKS cluster and IM IRSA roles via the infrastructure repo
   (`dhis2-sre/dhis2-infrastructure` Terraform).
3. Remove any EKS-only DNS records; restore DNS TTLs to normal values.
4. Keep the static IAM user scoped narrowly (S3 + KMS + SES only).

## Ongoing operations on the VM (post-migration)

The stateful services are now self-managed. Set up before/at cutover:

- **Postgres backups**: scheduled `pg_dump` to S3 (reuse the existing bucket or a separate one) plus
  VM snapshots. The DB is small, so frequent dumps are cheap.
- **Restart policy**: `restart: unless-stopped` on the compose services so the stack survives reboots.
- **Rebuild runbook**: since there is no HA, document how to rebuild the VM from image + latest DB
  dump. Target a short RTO.
- **Monitoring/alerting** on the VM (disk, container health, cert expiry).

---

## Appendix A - Dump the EKS Postgres

The EKS Postgres is a Bitnami pod (PostgreSQL 16; Bitnami chart `13.2.30`) in
`instance-manager-prod`, fronted by service `im-manager-postgresql-prod`. Its in-pod `pg_dump` is
v16, matching the VM target. Use the custom (`-Fc`) format for a clean, `--clean`-capable restore.

```bash
# Run from a machine with kubectl access to the EKS cluster.
NS=instance-manager-prod
POD=$(kubectl -n "$NS" get pod -l app.kubernetes.io/name=postgresql -o name | head -1)

# DB name/user default to instance-manager/instance-manager (confirm against the prod secret).
kubectl -n "$NS" exec "$POD" -- \
  sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -U instance-manager -d instance-manager -Fc' \
  > im-$(date +%Y%m%d-%H%M).dump
```

`$POSTGRES_PASSWORD` is already present inside the Bitnami pod's env, avoiding leaking the password
onto the host command line. Verify the file is non-trivial in size before proceeding.

## Appendix B - Restore into the VM Postgres

```bash
# Copy the dump to the VM, then load it into the compose Postgres container.
# Recreate the database clean so the restore is deterministic.
docker compose --profile prod up -d database
cat im-YYYYMMDD-HHMM.dump | docker compose exec -T database \
  pg_restore -U instance-manager -d instance-manager --clean --if-exists --no-owner

# If --clean warns because objects don't yet exist on a first load, that is expected; verify the
# final row counts instead of trusting exit noise.
```

Validate after restore:

```bash
docker compose exec -T database \
  psql -U instance-manager -d instance-manager -c \
  "SELECT (SELECT count(*) FROM users) AS users, (SELECT count(*) FROM clusters) AS clusters, (SELECT count(*) FROM groups) AS groups;"
```

Cross-check these counts against the same query run on EKS before the dump.

## Appendix C - Smoke test checklist

Against the VM (temp hostname during rehearsal / pre-DNS-flip cutover):

1. `GET /health` returns healthy.
2. Log in (email/password) via API; obtain a token.
3. List instances / deployments; counts match EKS.
4. Deploy a throwaway instance into a Hetzner group; wait for healthy; confirm its URL serves.
5. Destroy the throwaway instance; confirm cleanup.
6. Trigger a DB backup (S3 + KMS + pod-exec) and confirm the blob lands in `im-databases-prod`.
7. Check logs for `Environment variable not found` warnings and any KMS/S3 auth errors.

A clean run of 1-7 is the go/no-go signal for flipping DNS.

## Appendix D - Environment / secrets checklist for the VM

Carry these from the EKS config (SOPS-decrypted values in `helm/data/secrets/prod/` and the prod
`skaffold.yaml` env block). Bold entries are the must-match-exactly items from Gate B.

- **`INSTANCE_PARAMETER_ENCRYPTION_KEY`**, **`SOPS_KMS_ARN`**, **`PRIVATE_KEY`**,
  **`REFRESH_TOKEN_SECRET_KEY`**, `SESSION_SECRET`
- `DATABASE_HOST/PORT/USERNAME/PASSWORD/NAME` (now the VM's local Postgres)
- `REDIS_HOST/PORT`, `RABBITMQ_HOST/PORT/STREAM_PORT/USERNAME/PASSWORD`
- `HOSTNAME` (`api.im.dhis2.org`), `UI_URL`, `CORS_ALLOWED_ORIGINS`, `SAME_SITE_MODE=strict`,
  `COOKIE_SECURE=true`
- `S3_BUCKET=im-databases-prod`, `S3_REGION=eu-west-1`, `S3_ENDPOINT` **unset**
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` / `AWS_DEFAULT_REGION` (Gate C)
- `SMTP_HOST/PORT/USERNAME/PASSWORD`
- `GOOGLE_CLIENT_ID/SECRET/CALLBACK_URL` (callback must match `https://api.im.dhis2.org/...`)
- `DOCKER_HUB_USERNAME/PASSWORD`
- Token TTLs, `DEFAULT_TTL`, `PASSWORD_TOKEN_TTL`, `CLASSIFICATION`, `ENVIRONMENT`, `JAEGER_HOST/PORT`
- **Do not** set `GROUP_NAMES` / `GROUP_NAMESPACES` / `GROUP_HOSTNAMES` in a way that recreates
  groups on the nil default cluster (Gate A).
