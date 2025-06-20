# Getting Started

## Start local development environment with Docker

1. Start by copying the `.env.example` file into a new `.env` file:

```
cp .env.example .env
```

2. Create a private key:

```
make keys
```

3. Copy the private key contents, with actual newlines replaced by "\n", into the `PRIVATE_KEY` environment variable
   within the `.env` file:
   *This should work on macOS to copy the key contents*

```
cat rsa_private.pem | awk '{printf "%s\\n", $0}' | pbcopy
```

4. Initialize the environment and install dev dependencies:

```
make init
```

5. Start a development environment:

```
make dev
```

# Release

Releasing is done by creating a new release tag.

It's advised to generate the release log before doing so.

Example

```shell
make change-log
git commit CHANGELOG.md -m "chore: generate change log"
git tag --sort=-creatordate | head --lines=1              # Get the latest tag
git tag v0.53.0                                           # Use whichever tag you want to release
git push
```

# Add a group

* Add group in IM (either through the UI or by using the user script found [here](scripts/users/createGroup.sh)
* Update [values file](https://github.com/dhis2-sre/im-manager/blob/8cb9a5959e334b835188fa07e801996ff2410b7c/helm/chart/values.yaml#L12) or for an individual environment such as [prod](https://github.com/dhis2-sre/im-manager/blob/8cb9a5959e334b835188fa07e801996ff2410b7c/helm/data/values/prod/values.yaml#L1)
* Update the profiles section of the [skaffold file](https://github.com/dhis2-sre/im-manager/blob/8cb9a5959e334b835188fa07e801996ff2410b7c/skaffold.yaml#L96) to include the group
* Update backup schedule to include the group for either [dev](https://github.com/dhis2-sre/dhis2-infrastructure/blob/b9f53752ca9cb16883f2f78cae5fca42b4087b1f/modules/k8s/helm-backup-dev.tf#L1) or [prod](https://github.com/dhis2-sre/dhis2-infrastructure/blob/b9f53752ca9cb16883f2f78cae5fca42b4087b1f/modules/k8s/helm-backup-prod.tf#L1)

# Tracing

Tracing is implemented using [OpenTelemetry](https://opentelemetry.io/) and [Jaeger](https://www.jaegertracing.io/).

Tracing is enabled by default and configured for Gin and Gorm.

Forward the Jaeger UI to your local machine by running: `kubectl port-forward --namespace instance-manager-dev svc/jaeger-dev-query 16686:16686`
