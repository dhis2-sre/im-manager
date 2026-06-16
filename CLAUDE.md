# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Instance Manager (IM) is a Go web service that manages the lifecycle of DHIS2 instances on Kubernetes. Users (or
scripts) hit IM's HTTP API; IM authenticates them, then drives `helmfile sync` against a target k3s/EKS cluster to
install/upgrade/destroy DHIS2 deployments composed of one or more *stacks*.

## Common commands

```bash
make dev              # run im-manager + deps via docker-compose (dev profile)
docker compose up     # CLASSIFICATION=local; brings up IM + 3 in-Docker k3s clusters via Traefik
make test             # go test -race ./... — unit AND integration tests in one run
go test ./pkg/token/...                       # single package
go test ./pkg/token/... -run TestRefreshAccessToken   # single test
make check            # pre-commit: goimports, go-mod-tidy, golangci-lint, swagger validation, commitizen
make swagger          # regenerate swagger/swagger.yaml from code annotations
make keys             # generate the RSA private key required by .env's PRIVATE_KEY
skaffold run -p dev   # run im-manager + deps via skaffold (dev profile)
```

### Skaffold / kubeconfig

`skaffold run -p dev` must target the EKS production cluster. The correct kubeconfig is set in `.env`:

```
KUBECONFIG=$HOME/.kube/kubeconfig_instance-cluster-production
```

Run from `../im-tooling/` (which composes im-manager + im-web-client). The im-tooling `.envrc` computes `API_URL`, `UI_HOSTNAME`, etc. from `ENVIRONMENT`/`CLASSIFICATION` in `.env`, and sets `KUBECONFIG` to the EKS production cluster:

```bash
cd ../im-tooling && skaffold run -p dev
```

Do **not** use `im-hetzner-cluster.yaml` for skaffold deploys — that cluster lacks capacity for the feature namespace workloads.

`make test` runs everything together — `*_integration_test.go` files have **no build tag**. They use `pkg/inttest` which
spins real Postgres/RabbitMQ/Redis/Minio/localstack containers via testcontainers, so Docker must be running.

## Setup pitfalls

- `.env` is required. Copy `.env.example`, then run `make keys` and inline the key into `PRIVATE_KEY` (newlines as
  literal `\n`).
- `direnv` is expected; `.envrc` reads `.env` and computes `API_HOSTNAME`/`UI_URL` from `CLASSIFICATION` (`local`/
  `feature`/`dev`/`prod`).
- SOPS is mandatory for cluster kubeconfigs — exactly one of `SOPS_AGE_KEY` (local) or `SOPS_KMS_ARN` (AWS) must be set;
  the app refuses to start with neither or both.

## Architecture

### Request → cluster flow

1. `cmd/serve` wires everything: gin engine, JWT keys, Postgres (gorm), Redis (refresh tokens), RabbitMQ (events),
   MinIO/S3 (DB blobs), and reads dozens of env vars including `ACCESS_TOKEN_EXPIRATION_IN_SECONDS` /
   `REFRESH_ACCESS_TOKEN_EXPIRATION_IN_SECONDS`.
2. Each domain package under `pkg/` owns its slice (`router.go` → `handler.go` → `service.go` → `repository.go`).
   `internal/server` builds the gin engine; `internal/middleware` does authn; `internal/errdef` is the shared error
   taxonomy.
3. **Token model:** `pkg/token` issues short-lived RS256 access tokens and HS256 refresh tokens (refresh state in
   Redis). `RefreshAccessToken` is the load-bearing piece — it is called only at deploy-time (`pkg/instance/service.go`
   `DeployDeployment` and `UpdateInstance`) and the resulting JWT is embedded into pod env via `IM_ACCESS_TOKEN` (
   `pkg/instance/helmfile.go`). The pod's seed scripts curl back to the IM API with that token, so its lifetime must
   outlast the time between helm sync and the pod actually executing the curl.
4. **Inspector goroutine** (`pkg/inspector`) polls every 2 minutes to destroy instances past their TTL.

### Stacks (the deployable units)

`stacks/<name>/helmfile.yaml.gotmpl` + optional `seed.sh` define each stack (e.g. `dhis2-core`, `dhis2-db`, `minio`,
`pgadmin`, `chap-*`). Stacks are loaded from disk at startup and exposed via `pkg/stack`. Comment headers in each
helmfile carry metadata parsed by tests:

- `# consumedParameters:` — params required from upstream stacks in the same deployment
- `# stackParameters:` — params the stack defines for itself
- `# hostnameVariable:` / `# hostnamePattern:` — how downstream stacks reach this one

A *deployment* (`pkg/instance`) groups multiple instances (one per stack) topologically — `DeployDeployment` orders them
with `validateNoCycles` + `deploymentOrder`, refreshes the access token between each, then calls `helmfileService.sync`
which shells out to `helmfile` with a long list of injected env vars (`INSTANCE_NAME=<name>-<groupID>`,
`INSTANCE_NAMESPACE`, `IM_ACCESS_TOKEN`, AWS creds, ingress class, cert issuer, etc).

### Database seed pattern (important — easy to break)

Both `stacks/dhis2-db/seed.sh` (mounted as `initdb.scripts.seed.sh` under Bitnami postgres) and
`stacks/minio/seed-minio.sh` (a sidecar) curl `${HOSTNAME}/databases/${DATABASE_ID}` using `IM_ACCESS_TOKEN` and write
an idempotency marker only on success. **First-attempt failure → no marker → every restart retries with a now-expired
token → permanent CrashLoopBackOff.** When changing token TTLs or seed logic, keep this in mind: the marker is the
recovery mechanism.

### Helm chart vs. helmfile stacks (don't confuse them)

- `helm/chart/` is the chart for **IM itself** — env-specific values live in `helm/data/values/{dev,feature,prod}`,
  secrets in `helm/data/secrets/`. Skaffold (`skaffold.yaml`) deploys it.
- `stacks/` are the helmfile stacks IM **deploys to other clusters** for DHIS2 instances. Different system entirely.

## Migrations

See [README.md#migrations](README.md#migrations). Any model change or new stack parameter that leaves existing rows inconsistent requires a migration.

## Release

See [README.md#release](README.md#release) for release steps.

GitHub Actions (`.github/workflows/build-test-deploy.yaml`) delegates to a reusable workflow in
`dhis2-sre/gha-workflows` for image build + deploy. PR events deploy a feature env into the `instance-manager-feature`
namespace; PR close tears it down via `delete-env.yaml`.

## Operational scripts

`scripts/` holds shell scripts that exercise the API (`scripts/instances/`, `scripts/users/`, etc). Each subdirectory
has its own `.env` for `IM_HOST`/`USER_EMAIL`/`PASSWORD` and reuses `auth.sh` to cache an access token.
`scripts/util/find-orphaned-helm-releases.py` is a helm-vs-IM-DB drift check that runs daily as a workflow and posts to
Slack.

## Commiting, pushing and creating PRs

### Don't

* Use em-dashes
* Add Claude as a co-author
* Include Claude's name in the PR description
* Insert line breaks in the PR description
* Insert line breaks in the commit message

### Do

* Keep the PR description short
* Keep the commit message short
