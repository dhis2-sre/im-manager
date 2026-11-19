# CLAUDE.md

Traps that the code does not show. Everything else is in the code, `README.md` and `docs/testing.md`.

## Before handing work over

- Run `make check`. CI runs it before the tests, so a failure there hides the test results.
- Run `make swagger` after changing handler annotations and commit the regenerated `swagger/swagger.yaml`.

## Deployment

- The servers that run IM use `docker-compose.yml` verbatim, so a change to that file is a change to prod. Ports, builds
  and local stand-ins belong in `docker-compose.dev.yml` and `docker-compose.k3s.yml`.
- Master deploys to dev, a version tag to prod, and a pull request labelled `deploy` to a feature environment named
  after its head branch. A branch of the same name in im-web-client shares that environment, so a change spanning the
  API and the client uses one branch name in both repositories.
- `make release` tags and pushes master, which deploys prod. Run it only when asked to.
- `helm/chart`, `helm/data` and `skaffold.yaml` are the former EKS deployment of IM itself. Nothing deploys them
  anymore, so do not keep them in sync with other changes.

## Stacks

- Parameter values are stored per instance, so adding, removing or renaming a stack parameter, or changing a default
  such as `CHART_VERSION`, needs a migration. Without one for a removed parameter, every update of an affected
  deployment fails with `parameter not found on stack` while deploys keep working.
- `scripts/instances/deploy-dhis2.sh` is checked out at the latest tag by pipelines outside this repository, which
  pass parameters as environment variables. A parameter it stops forwarding silently falls back to the stack default,
  and a fix reaches the pipelines only with a new release.

## Committing, pushing and creating PRs

- Use Conventional Commits for commit messages and pull request titles. `CHANGELOG.md` is generated from them, so
  never edit it by hand.
- Keep commit messages and pull request descriptions short, and do not hard-wrap their lines.
- Do not use em-dashes.
- Do not add Claude as a co-author or mention Claude in the pull request description.
