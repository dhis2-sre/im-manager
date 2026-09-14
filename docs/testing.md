# Running and measuring tests

```sh
make test                         # entire suite, including Kubernetes
make test-integration             # unit/integration tests without Kubernetes
make test-e2e                     # TestInstanceHandler and all its subtests
make test TEST_FLAGS='-shuffle=on' # forwards additional go test flags
make test-integration TEST_PARALLEL=4
```

Docker is required. Kubernetes tests additionally need Helm and Helmfile on PATH.
All make targets disable Go's test-result cache with `-count=1`; the build cache
still applies. The default package concurrency is four; use `TEST_PARALLEL=2` to retain the
previous setting on constrained Docker hosts.

For a focused test, direct invocations continue to work:

```sh
go test -race -count=1 ./pkg/database
```

## Service ownership and isolation

`internal/testenv/cmd` starts PostgreSQL, Redis, LocalStack S3 and MinIO
concurrently, exports their addresses in `IM_TEST_SERVICES`, runs `go test`, then
terminates its containers. Each invocation owns fresh containers with dynamically
allocated ports. Concurrent invocations do not share services or persistent data.
RabbitMQ and Kubernetes retain their existing lifetimes.

The runner and package `TestMain` configure a fixed test-only
`INSTANCE_PARAMETER_ENCRYPTION_KEY` before starting services or tests. The template
migrations require this variable even on an empty database. Tests do not depend on
an application `.env` file or inherit its encryption key.

Each call to a fixture helper allocates isolated data:

| Helper | Isolation | Cleanup |
|---|---|---|
| `SetupDB(t)` | Database cloned from a migrated template | Close connection pool, drop database |
| `SetupRedis(t)` | Exclusively leased logical database | Flush that database, close client, return lease |
| `SetupS3(t)` | Unique LocalStack bucket returned as `Bucket` | Delete objects, abort unfinished uploads, delete bucket |
| `SetupMinIO(t)` | Unique MinIO bucket returned with client | Same bucket cleanup |

PostgreSQL migrations, extensions and indexes run once during template creation.
The template's connections are then closed and new connections to it disabled.
Fixtures connect to clones using `storage.ConnectDatabase`; production startup
still calls `storage.NewDatabase` and runs migrations. Tests specifically covering
migrations should use a fresh database and the production initialization path.

Redis is configured with 64 logical databases. DB 0 holds an atomic lease pool;
DBs 1–63 are available to fixtures across all Go test processes. Exhaustion waits
up to 30 seconds and fails explicitly. A database whose cleanup fails is not
returned to the pool. Tests must not run FLUSHALL or alter global Redis settings.

Fixture ownership follows Go's test lifetime, including all subtests. Existing
parent fixtures continue to be shared by their subtests. Stop background workers
before fixture cleanup, using later-registered `t.Cleanup` callbacks where needed.
S3 fixture buckets do not enable versioning; tests of versioning or global service
configuration need an appropriately specialized fixture.

Without `IM_TEST_SERVICES`, helpers lazily start one service instance per package
using `sync.OnceValues`. Packages using these helpers must include:

```go
func TestMain(m *testing.M) { inttest.Main(m) }
```

This closes package-local containers after every test and cleanup finishes. Shared
containers are never attached to the first test's `t.Cleanup`.

The MinIO release remains `RELEASE.2025-01-20T14-49-07Z`, sourced from Quay because
the original Docker Hub reference returned pull-access-denied during measurement.
PostgreSQL 16.2, Redis 6.0.9 and LocalStack 3.0.0 retain their previous versions.

## Comparing performance

Capture wall time as well as package durations. Packages overlap, so their elapsed
times must not be summed to estimate suite wall time. Keep `-race`, package
concurrency, test selection, toolchain and image versions the same. Warm build/image
caches on both revisions; disable the test-result cache. Run revisions sequentially
so they do not contend for Docker resources.

```sh
# Baseline revision, with the same MinIO registry correction:
/usr/bin/time -p go test -race -p 2 -count=1 -json \
  -skip '^TestInstanceHandler$' ./... > baseline.json

# Refactored revision:
/usr/bin/time -p go run ./internal/testenv/cmd -- \
  -race -p 4 -count=1 -json -skip '^TestInstanceHandler$' ./... > shared.json
```

The runner reports service startup, Go test execution (including compilation),
cleanup and total runner time. Compilation of the runner itself occurs before its
timer starts; the enclosing command or Actions step includes that cost.

The `Test performance` workflow runs separate integration and Kubernetes jobs for
relevant draft PRs, and also supports manual dispatch. Each job uploads JSON test
results and writes phase timings to the Actions summary. It uses disposable test
services and does not require application/deployment secrets. The existing shared
build workflow still runs the full `make test` gate. Independent measurement jobs
are intentionally limited to drafts/manual runs to avoid permanently duplicating
that gate. Moving the gate out of the reusable workflow requires a coordinated
change to `dhis2-sre/gha-workflows` so deployments still depend on all tests passing.

## Initial CI baseline

The latest successful master run inspected was
[34339403680](https://github.com/dhis2-sre/im-manager/actions/runs/34339403680):

| Measurement | Duration |
|---|---:|
| Build job | 16m43s |
| Image build step | 4m58s |
| Unit/integration test step | 4m52s |
| `pkg/instance`, including Kubernetes | 4m10s |

The slower [34355983640](https://github.com/dhis2-sre/im-manager/actions/runs/34355983640)
run spent 9m11s in the test step and 6m35s building the image. It used a different
branch, so it provides context rather than a controlled before/after comparison.
Halving the test step alone cannot halve a workflow with substantial serial build,
setup and deployment work.

## Local results (2026-09-14)

Measured on macOS/arm64 with Go 1.26.2 and Docker Desktop. Baseline revision:
`a2446d6a`, with only the MinIO registry corrected so all integration tests could
run. Tests ran sequentially across revisions, with warmed dependency/image caches,
`-race`, `-count=1`, and `TestInstanceHandler` excluded. Wall times include the
runner's compilation, service startup and cleanup. The refactor also runs the new
fixture-isolation tests.

| Configuration | Wall time | Change from baseline |
|---|---:|---:|
| Original helpers, `-p 2` | 107.82s | — |
| Shared services, `-p 2` | 90.97s | 15.6% faster |
| Shared services, `-p 4` (new default) | 63.28s | 41.3% faster |

These are individual local runs, not a statistical estimate of CI performance.
First measured runs were 114.59s before and 100.75s after at `-p 2`; those included
additional compilation/cache warmup. The four-package run spent approximately
three seconds starting services and 1.3 seconds cleaning them up.

At the same `-p 2` setting, package timings changed as follows. Shared service
startup is outside these package durations and is included in the wall times above.

| Package | Original | Shared |
|---|---:|---:|
| cluster | 9.39s | 3.34s |
| database | 14.46s | 3.92s |
| event | 21.84s | 19.26s |
| group | 9.21s | 5.95s |
| instance, excluding Kubernetes | 16.15s | 3.10s |
| token | 4.81s | 2.02s |
| user | 39.47s | 32.59s |

The attempted full baseline failed when its local Kubernetes node acquired a
`node.kubernetes.io/disk-pressure` taint. That run is not a valid end-to-end timing
baseline. The draft PR's Linux measurement jobs provide Kubernetes validation and
CI timings; no full-workflow speedup is claimed from the local integration numbers.

## Validation

The integration suite passes with `-race` at both two- and four-package
concurrency. A two-pass run with `-shuffle=1789379141114039000 -count=2` also passes.
That run exposed an existing package-global user counter; it now belongs to
`TestUserHandler` so subsequent runs count only users in their own database.
The isolation tests cover concurrent cloned databases, cross-process Redis
allocation, bucket separation, database/bucket cleanup and unfinished S3 uploads.
The runner's partial-startup failure path was also checked for container cleanup.
