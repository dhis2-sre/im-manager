# Instance Manager

Instance Manager is a web application that manages the lifecycle of DHIS2 instances.

## Getting Started

If you just want to see what the application looks like you can find some screenshots [here](./docs/screenshots).

### Start local development environment with Docker

1. Start by copying the `.env.example` file into a new `.env` file:

```shell
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

## Encryption

Cluster kubeconfigs are encrypted at rest using [SOPS](https://github.com/getsops/sops). Exactly one encryption backend must be configured. The application fails at startup if neither (or both) are set.

### Local development — age

Generate an age key pair:

```shell
age-keygen
```

Copy the private key line (starts with `AGE-SECRET-KEY-1...`) into your `.env`:

```
SOPS_AGE_KEY=AGE-SECRET-KEY-1...
```

### AWS environments — KMS

Set the KMS key ARN via `SOPS_KMS_ARN`. AWS credentials are provided automatically through the pod's IAM service account role — no explicit credential configuration is needed.

# Release

Releasing is done by creating a new release tag.

It's advised to generate the release log before doing so.

Example

```shell
git tag --sort=-creatordate | head --lines=1              # Get the latest tag
git tag v0.53.0                                           # Use whichever tag you want to release
make change-log
git commit CHANGELOG.md -m "chore: generate change log"
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

## Let's Encrypt on the outer Traefik

Only relevant when deploying the compose stack against a real public domain.

Prerequisites on the deploy host:

- `A` records for the UI and API hostnames pointing at the host's public IP.
- Ports `80` and `443` reachable from the internet (HTTP-01 challenge uses `:80`).

Apply these edits on the deploy host; do not commit them.

**`docker-compose.k3s.yml` — `traefik` service:**

```yaml
command:
  - --entrypoints.web.address=:80
  - --entrypoints.web.http.redirections.entryPoint.to=websecure
  - --entrypoints.web.http.redirections.entryPoint.scheme=https
  - --entrypoints.websecure.address=:443
  - --providers.file.filename=/etc/traefik/dynamic.yml
  - --certificatesResolvers.letsencrypt.acme.email=you@example.org
  - --certificatesResolvers.letsencrypt.acme.storage=/acme/acme.json
  - --certificatesResolvers.letsencrypt.acme.caServer=https://acme-staging-v02.api.letsencrypt.org/directory
  - --certificatesResolvers.letsencrypt.acme.httpChallenge.entryPoint=web
ports:
  - "80:80"
  - "443:443"
volumes:
  - ./traefik-dynamic.yml:/etc/traefik/dynamic.yml:ro
  - traefik-acme:/acme
```

And declare the volume at the bottom of the file:

```yaml
volumes:
  traefik-acme:
```

**`traefik-dynamic.yml` - `im-web-client` and `im-api` routers:**

```yaml
im-web-client:
  rule: "Host(`im.example.org`)"
  service: im-web-client
  entryPoints: [web, websecure]
  tls:
    certResolver: letsencrypt
im-api:
  rule: "Host(`api.im.example.org`)"
  service: im-api
  entryPoints: [web, websecure]
  tls:
    certResolver: letsencrypt
```

Start with LE **staging** (shown above) to validate the flow - staging has very generous rate limits, prod does not.

Verify:

```shell
docker compose up -d traefik
docker logs im-manager-traefik-1 2>&1 | grep -i "certificate obtained"
openssl s_client -connect im.example.org:443 -servername im.example.org </dev/null 2>/dev/null \
  | openssl x509 -noout -issuer
# expect: issuer=C=US, O=(STAGING) Let's Encrypt, CN=(STAGING) ...
curl -I http://im.example.org/            # expect: 301 → https
```

Promote to production LE once staging works end-to-end:

```shell
# change caServer to:
# https://acme-v02.api.letsencrypt.org/directory
# then wipe staging state so prod issues fresh certs:
docker compose down traefik
docker volume rm im-manager_traefik-acme
docker compose up -d traefik
```
