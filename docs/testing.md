# Running and measuring tests

```sh
make test                         # entire suite, including Kubernetes
make test-integration             # unit/integration tests without Kubernetes
make test-e2e                     # TestInstanceHandler and all its subtests
make test TEST_FLAGS='-shuffle=on' # forwards additional go test flags
make test-integration TEST_PARALLEL=4
```

Docker is required. Kubernetes tests additionally need Helm, Helmfile and SOPS on PATH.
Database backups execute pg_dump in the PostgreSQL pod; no host pg_dump is required.
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
RabbitMQ shares the runner lifecycle too; Kubernetes retains its existing lifetime.

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
| `SetupRabbitStream(t)` / `SetupRabbitMQAMQP(t)` | Unique RabbitMQ virtual host | Close clients, delete virtual host |

PostgreSQL migrations, extensions and indexes run once during template creation.
The template's connections are then closed and new connections to it disabled.
Fixtures connect to clones using `storage.ConnectDatabase`; production startup
still calls `storage.NewDatabase` and runs migrations. Tests specifically covering
migrations should use a fresh database and the production initialization path.

Redis is configured with 128 logical databases. DB 0 holds an atomic lease pool;
DBs 1–127 are available to fixtures across all Go test processes. Exhaustion waits
up to 30 seconds and fails explicitly. A database whose cleanup fails is not
returned to the pool. Tests must not run FLUSHALL or alter global Redis settings.

The test broker uses the multiarchitecture official RabbitMQ 3.13.7 management
image, pinned by manifest digest. Plugins are enabled before its single startup;
this preserves the previous broker version while avoiding Bitnami bootstrap restarts.

RabbitMQ fixtures share one broker and use separate virtual hosts, so stream and
queue names can repeat without sharing messages or consumer offsets. Fixture
cleanup closes stream environments or AMQP channels/connections before deleting
the virtual host. Register producer/consumer cleanup after fixture setup so it
runs first. Tests that change broker-wide configuration or restart the broker
need a separately owned broker. Kubernetes-only runs omit RabbitMQ.

Fixture ownership follows Go's test lifetime, including all subtests. Existing
parent fixtures continue to be shared by their subtests. Stop background workers
before fixture cleanup, using later-registered `t.Cleanup` callbacks where needed.
S3 fixture buckets do not enable versioning; tests of versioning or global service
configuration need an appropriately specialized fixture.

Fixture helpers access services through the typed `testenv.Shared.Postgres()`,
`Redis()`, `S3()`, `MinIO()`, and `RabbitMQ()` methods. Missing or malformed runner configuration
fails explicitly instead of starting replacement containers.

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

The `Test performance` workflow runs separate integration and Kubernetes jobs on
manual dispatch. It does not duplicate the full gate on every PR update. Each job uploads JSON test
results and writes phase timings to the Actions summary. It uses disposable test
services and does not require application/deployment secrets. The existing shared
build workflow still runs the full `make test` gate. Independent measurement jobs
are intentionally manual to avoid duplicating that gate. Moving the gate out of the reusable workflow requires a coordinated
change to `dhis2-sre/gha-workflows` so deployments still depend on all tests passing.

## CI baselines

The latest successful master run inspected was
[34339403680](https://github.com/dhis2-sre/im-manager/actions/runs/34339403680):

| Measurement | Duration |
|---|---:|
| Build job | 16m43s |
| Image build step | 4m58s |
| Unit/integration test step | 4m52s |
| `pkg/instance`, including Kubernetes | 4m10s |

The slower [34355983640](https://github.com/dhis2-sre/im-manager/actions/runs/34355983640)
run on `version-3.0` commit `29d1cb9b` spent 9m11s in the test step and 6m35s
building the image. The refactor is now based on that same revision. These are
historical timings; runner load, dependency versions and cache state may differ.
Halving the test step alone cannot halve a workflow with substantial serial build,
setup and deployment work.

## Initial local results on master (2026-09-14)

These measurements predate the rebase onto `version-3.0`; they do not measure its
updated Kubernetes/CNPG tests. Measured on macOS/arm64 with Go 1.26.2 and Docker
Desktop. Baseline revision:
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

## Setup and CI caches

`make init` preserves pre-commit environments and installs pinned tool versions.
The tool installer checks installed Go binary metadata before reinstalling a tool.
The sequential build workflow opts into separate module, compilation and setup
caches. Compilation keys include source content and restore compatible older
entries; Go still validates cached objects and the tests still use `-count=1`.
The workflow reference is pinned to the companion cache-change PR while it is
being evaluated; checks, image build, smoke tests and the full test suite keep
their existing order.

## Native ARM64 Kubernetes validation

Chart 1.0.1 has two AMD64-only database images: the CNPG operand
`ghcr.io/cloudnative-pg/postgis:17-3.5` and the seed client
`dhis2/postgresql-curl:17`. Pruning Docker cleared the observed disk pressure,
but cannot resolve either image's missing ARM64 platform.

The candidate [chart update](https://github.com/dhis2-sre/dhis2-core-chart/pull/89)
keeps PostgreSQL 17 and uses a digest-pinned PostGIS 3.6 Bookworm manifest with
AMD64 and ARM64 variants. Its
[seed image update](https://github.com/dhis2-sre/bitnami-postgresql-curl/pull/3)
uses the official PostgreSQL 17 Bookworm base plus curl, under the postgres user.
The seed workflow builds and smoke tests both platforms before publication.

Release order: publish `dhis2/postgresql-curl:17-bookworm`, remove the chart PR's
temporary candidate-image import steps, release chart 1.1.0, then update
im-manager's CHART_VERSION default. Until then, im-manager retains released chart
1.0.1; ordinary native ARM64 Kubernetes runs still need the candidate chart.
The changes do not parallelize CI gates.

Local validation on 2026-09-14 used a packaged candidate chart in place of the
released chart reference and imported the locally built seed image into the
disposable k3s container. The complete `make test-e2e` passed with `-race`:
3m10s for the Go test phase and 3m24s including service setup and cleanup.
PostgreSQL reported 17.11 on aarch64 and PostGIS reported 3.6.4. Database
save/restore, MinIO backup and filesystem backup all passed. The node reported
DiskPressure=False and MemoryPressure=False. This is a compatibility measurement,
not a controlled speed comparison against a previously passing local baseline.
The final full `make test` run also passed with `-race -p 4 -count=1`, taking
3m33s including shared-service setup and cleanup. Other local Docker work was
active during these measurements.

If an image was previously pulled for AMD64, Docker can reuse that cached variant
on an ARM64 host. The shared MinIO release already provides both architectures;
verify the selected image with `docker image inspect IMAGE` and refresh its native
variant with `docker pull --platform linux/arm64 IMAGE` on an ARM64 Docker daemon.
The diagnosed cached AMD64 MinIO process crashed under emulation; its native ARM64
binary starts successfully. Use the Docker daemon's architecture when the daemon
is remote, rather than assuming it matches the Go host.

### Kubernetes volume cleanup

Gnomock debug mode disables Docker auto-removal, and this fork's explicit Stop
removes containers without their anonymous volumes. Repeated Kubernetes runs
therefore accumulate several GB per run and eventually cause disk pressure again.
The test helper leaves debug mode disabled so Docker removes those volumes with
the container; Kubernetes logs can still be inspected while the test is running.
This cleanup applies only to the test container's anonymous volumes, not named
volumes or other local workloads.
