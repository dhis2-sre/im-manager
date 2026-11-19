# CLAUDE.md

Instance Manager (IM) is a Go web service that manages the lifecycle of DHIS2 instances on Kubernetes. A *deployment*
groups one instance per *stack*, and IM deploys it by running `helmfile sync` against the cluster of the deployment's
group. The web client lives in [dhis2-sre/im-web-client](https://github.com/dhis2-sre/im-web-client).

This file only records what the code does not show at a glance. `README.md` covers local setup, clusters, migrations and
releases, and `docs/testing.md` covers the test runner and its fixtures.

## Commands

```bash
make init                # install the pinned tools and the pre-commit hooks
make check               # pre-commit on all files: goimports, go mod tidy, golangci-lint, swagger validation
make test                # the entire suite, including the Kubernetes tests
make test-integration    # everything except the Kubernetes tests
make test-e2e            # only the Kubernetes tests (TestInstanceHandler in pkg/instance)
go test -race -count=1 ./pkg/token/... -run TestRefreshAccessToken   # one package or one test
make swagger             # regenerate swagger/swagger.yaml from the handler annotations
```

- Tests need a running Docker daemon and nothing else from this repository: they start their own containers and do not
  read `.env`. The Kubernetes tests also need `helm`, `helmfile` and `sops` on the `PATH`, start a k3s container and
  deploy real DHIS2 pods, so they are slow and can time out on a small machine. Prefer `make test-integration` or a
  single package locally and let CI judge the Kubernetes tests.
- CI runs `make check` before `make test`, so a lint or swagger failure hides the test results. Run `make check` before
  handing work over.
- `swagger/swagger.yaml` is generated and checked in. Run `make swagger` after changing handler annotations or
  request and response types, and commit the result.
- Tools are installed at pinned versions by `scripts/install-test-tools.sh` and `.pre-commit-config.yaml`. Keep new
  tools pinned as well, because an unpinned tool has turned every pull request red before.
- Compose commands (`docker compose ...`, `make dev`, `make smoke-test`) rely on `COMPOSE_FILE` and `COMPOSE_PROFILES`
  from `.envrc`. A shell without the direnv hook needs `direnv exec . <command>`. A new git worktree has no `.env`.

## Where IM runs

- Every environment (prod, dev and the feature environments) runs as Docker Compose on the im-vm servers, which are
  managed in `dhis2-sre/dhis2-infrastructure` under `services/im-vm`. The servers use `docker-compose.yml` verbatim, so
  a change to that file is a change to prod. Ports, builds and local stand-ins belong in `docker-compose.dev.yml` and
  `docker-compose.k3s.yml`.
- `build-test-deploy.yaml` builds and pushes the image, then `deploy-im-vm.yaml` deploys it: master goes to dev, a
  version tag goes to prod, and a pull request labelled `deploy` gets a feature environment named after its head
  branch. Both workflows are thin callers of reusable workflows in `dhis2-sre/gha-workflows`, which is where the
  logic lives.
- A branch of the same name in im-web-client shares the feature environment, and `e2e.yaml` runs the Playwright suite
  of im-web-client from that branch when it exists. A change that spans the API and the client therefore uses the same
  branch name in both repositories.
- `make release version=<version>` tags master and pushes it, which deploys prod. Run it only when asked to.
- `helm/chart`, `helm/data` and `skaffold.yaml` are the former EKS deployment of IM itself. No pipeline deploys them
  anymore, and they have nothing to do with `stacks/`.

## Stacks

- A stack is defined in two places that must agree: the Go definition in `pkg/stack/` (parameters, defaults,
  validators and companions, registered in `stack.All`) and `stacks/<name>/helmfile.yaml.gotmpl` with its
  `# consumedParameters:`, `# stackParameters:` and `# hostnameVariable:` header comments. The tests in
  `pkg/stack/stack_definitions_test.go` fail when the two drift apart.
- Parameter values are stored per instance, so the definition alone does not change what existing instances have.
  A new parameter needs a migration that backfills it. A removed or renamed parameter needs a migration that deletes
  or renames the stored rows, otherwise every update of an affected deployment fails with
  `parameter not found on stack` while deploys keep working. A changed default, such as `CHART_VERSION`, also needs a
  migration to reach existing instances. See [README.md#migrations](README.md#migrations).
- `stacks/<name>/parameters/<classification>.yaml` holds SOPS-encrypted values that are passed to every helmfile run of
  that stack. A missing file is ignored silently, which shows up as a feature that is blank in one environment only.
- `dhis2-v2` installs the `dhis2` chart from `dhis2-sre/dhis2-core-chart`. The seed job, its marker and the probes are
  part of that chart, so a change there usually takes a chart release, a new `CHART_VERSION` default and a migration.
- The database is seeded once. At deploy time IM creates download links that expire after 30 minutes and passes them as
  `DATABASE_DOWNLOAD_URL` and `FILESTORE_DOWNLOAD_URL`, and the chart's marker-guarded job runs `seed.sh` with them.
  This is why `DATABASE_ID` cannot be changed after the first deploy.

## Scripts

- `scripts/instances/deploy-dhis2.sh` is used by Jenkins and GitHub pipelines outside this repository, which check it
  out at the latest tag and pass parameters as environment variables. Treat those variables as a public interface: a
  parameter the script stops forwarding silently falls back to the stack default, and a fix reaches the pipelines only
  with a new release.
- The other scripts under `scripts/<area>/` call the API with the credentials in that directory's `.env`.
- `scripts/util/*.py` are the drift checks that the `find-*.yaml` workflows run on a schedule and report to Slack.
- `cmd/migrate-v3` and `internal/migratev3` are a one-off migration command that is due for removal. Do not build on
  them.

## Conventions

- Spell names out in code, scripts and paths instead of abbreviating them, and use long-form command options
  (`--namespace` rather than `-n`) in scripts and workflows.
- Write comments, error messages and documentation as full sentences that explain why. Let the code say what it does.

## Committing, pushing and creating PRs

- Commit messages follow Conventional Commits (`feat:`, `fix:`, `chore:`, ...), which the commitizen hook enforces.
  Pull requests are normally squash-merged and `CHANGELOG.md` is generated from the resulting subjects, so the pull
  request title needs the same format. Never edit `CHANGELOG.md` by hand.
- Keep commit messages and pull request descriptions short.
- Do not hard-wrap lines in commit messages or pull request descriptions.
- Do not use em-dashes.
- Do not add Claude as a co-author or mention Claude in the pull request description.
