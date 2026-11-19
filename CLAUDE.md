# CLAUDE.md

Instance Manager (IM) is a Go web service that manages DHIS2 instances on Kubernetes. A *deployment* groups one instance
per *stack*, and IM deploys it by running `helmfile sync` against the cluster of the deployment's group. The web client
lives in [dhis2-sre/im-web-client](https://github.com/dhis2-sre/im-web-client). Setup, migrations and releases are
described in `README.md`, the test runner in `docs/testing.md`.

## Testing and checks

```bash
make check               # pre-commit on all files; CI runs it before the tests, so a failure hides the test results
make test-integration    # everything except the Kubernetes tests
make test-e2e            # only the Kubernetes tests (TestInstanceHandler in pkg/instance)
go test -race -count=1 ./pkg/token/... -run TestRefreshAccessToken
make swagger             # regenerate the checked-in swagger/swagger.yaml after changing handler annotations
```

- Tests only need Docker. They start their own containers and do not read `.env`.
- The Kubernetes tests deploy real DHIS2 pods into a k3s container and can time out on a small machine. Run the other
  tests locally and let CI judge those.
- Compose commands rely on variables from `.envrc`. A shell without the direnv hook needs `direnv exec . <command>`.

## Where IM runs

- Every environment runs as Docker Compose on the im-vm servers managed in `dhis2-sre/dhis2-infrastructure`. They use
  `docker-compose.yml` verbatim, so a change to that file is a change to prod. Ports, builds and local stand-ins belong
  in `docker-compose.dev.yml` and `docker-compose.k3s.yml`.
- Master deploys to dev, a version tag to prod, and a pull request labelled `deploy` to a feature environment named
  after its head branch. The logic lives in the reusable workflows of `dhis2-sre/gha-workflows`.
- A branch of the same name in im-web-client shares the feature environment and its Playwright suite runs against it,
  so a change spanning the API and the client uses one branch name in both repositories.
- `make release` tags and pushes master, which deploys prod. Run it only when asked to.
- `helm/chart`, `helm/data` and `skaffold.yaml` are the former EKS deployment of IM itself. Nothing deploys them
  anymore, and they are unrelated to `stacks/`.

## Stacks

- A stack is defined twice: in Go under `pkg/stack/` and in `stacks/<name>/helmfile.yaml.gotmpl` with its header
  comments. `pkg/stack/stack_definitions_test.go` fails when the two drift apart.
- Parameter values are stored per instance, so adding, removing or renaming a parameter, or changing a default such as
  `CHART_VERSION`, needs a migration. Without one for a removed parameter, every update of an affected deployment fails
  with `parameter not found on stack` while deploys keep working.
- `stacks/<name>/parameters/<classification>.yaml` holds SOPS-encrypted values passed to every helmfile run of that
  stack. A missing file is ignored silently.
- `dhis2-v2` installs the chart from `dhis2-sre/dhis2-core-chart`, which owns the seed job, its marker and the probes.
  The job seeds the database once, from download links that IM creates at deploy time and that expire after 30
  minutes, so `DATABASE_ID` cannot be changed after the first deploy.
- `scripts/instances/deploy-dhis2.sh` is checked out at the latest tag by pipelines outside this repository, which
  pass parameters as environment variables. A parameter it stops forwarding silently falls back to the stack default,
  and a fix reaches the pipelines only with a new release.

## Committing, pushing and creating PRs

- Use Conventional Commits for commit messages and pull request titles. `CHANGELOG.md` is generated from them, so
  never edit it by hand.
- Keep commit messages and pull request descriptions short, and do not hard-wrap their lines.
- Do not use em-dashes.
- Do not add Claude as a co-author or mention Claude in the pull request description.
