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

`internal/testenv/cmd` starts PostgreSQL, Redis, LocalStack S3, MinIO and RabbitMQ
concurrently, exports their addresses in `IM_TEST_SERVICES`, runs `go test`, then
terminates its containers. Each invocation owns fresh containers with dynamically
allocated ports. Concurrent invocations do not share services or persistent data.
Kubernetes tests own a separate k3s cluster and chart-managed MinIO pods.
Their event publisher is a no-op; those deployments do not use the shared broker.

All runner invocations use `StartAll()`, including `make test-e2e`. The Makefile
selects which tests run through standard `go test` arguments.

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
| `SetupMinIO(t)` | Unique MinIO bucket returned as `Bucket` | Same bucket cleanup |
| `SetupRabbitStream(t)` / `SetupRabbitMQAMQP(t)` | Unique RabbitMQ virtual host | Close clients, delete virtual host |

S3 and MinIO fixtures expose `Client` and `Bucket` together. S3 convenience methods
`GetObject(t, key)` and `TryGetObject(key)` always use that fixture's bucket. Cleanup
is registered automatically with `t.Cleanup`: it runs after the calling test and
all its subtests finish, while the shared server stays running. Tests must stop
any workers and close producers or consumers before their fixture is cleaned up.

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
need a separately owned broker.

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

Service image versions are centralized in `internal/testenv/environment.go`.

## Kubernetes requirements

The default DHIS2 chart is 1.1.0. Its PostgreSQL/PostGIS and seed images support
Linux AMD64 and ARM64, so the same Kubernetes suite runs locally and in CI.
Helm must be able to download the chart and Docker must be able to pull its images.

The k3s helper removes its container and anonymous volumes after the test.
Allow enough Docker disk space for Kubernetes images and volumes; a node under
disk pressure can evict pods and fail deployment tests. Cleanup only removes the
test's own resources.

## Setup and CI caches

`make init` preserves pre-commit environments and installs pinned tool versions.
The installer checks installed Go binary metadata before reinstalling a tool.
The sequential build workflow opts into separate module, compilation and setup
caches. Compilation keys include source content and restore compatible older
entries; Go still validates cached objects and tests still use `-count=1`.
Checks, image build, smoke tests and the full test suite retain their existing order.

## Comparing performance

Capture wall time as well as package durations. Packages overlap, so their elapsed
times must not be summed to estimate suite wall time. Keep race detection, package
concurrency, test selection, toolchain and image versions the same. Warm build and
image caches on both revisions, disable the test-result cache, and run revisions
sequentially so they do not contend for Docker resources.

```sh
/usr/bin/time -p make --silent test-integration TEST_PARALLEL=4 TEST_FLAGS=-json > integration.json
/usr/bin/time -p make --silent test-e2e TEST_FLAGS=-json > e2e.json
/usr/bin/time -p make --silent test TEST_PARALLEL=4 TEST_FLAGS=-json > full.json
```

The runner reports service startup, Go test execution (including compilation),
cleanup and total runner time to stderr and the GitHub Actions step summary.
Compilation of the runner itself occurs before its timer starts; the enclosing
command or Actions step includes that cost.

Measured results and their limitations are recorded in
[PR #1764](https://github.com/dhis2-sre/im-manager/pull/1764).
