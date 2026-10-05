# migrate-v3

One-off command moving the deployments made of the stacks version 3.0 removed (`dhis2-db`, `minio`, `dhis2-core` and an optional `pgadmin`) onto the `dhis2-v2` umbrella stack. It is removed, together with `internal/migratev3` and its `v3_migration_states` table, once production has been migrated.

Postgres moves from the Bitnami StatefulSet to CloudNativePG, so the data is moved rather than relabelled: every deployment is snapshotted to S3 as a database record of its group, rewritten to a `dhis2-v2` instance seeded from that snapshot, deployed by instance manager itself, and pointed back at its original `DATABASE_ID` once seeded.

## Phases

| Phase | Runs while | Does |
|---|---|---|
| `plan` | the old version serves | Lists what would be migrated, the database sizes and what blocks any deployment. Changes nothing, not even the schema. |
| `migrate` | nothing serves | Runs the 3.0 schema migrations, then per deployment, largest database first: scales the old core to 0, dumps the database and archives the file store to S3, rewrites the rows to `dhis2-v2`, uninstalls the old core, MinIO and pgAdmin releases. The old database is only scaled to 0 and kept as a fallback. |
| `deploy` | 3.0 serves | Deploys each deployment through the API, waits until the seed job has completed and the core is available (deploying a second time if helm timed out on the seed), restores the original `DATABASE_ID`, uninstalls the old database and pauses what was paused before. |
| `cleanup` | 3.0 serves, after `--grace` | Deletes the snapshots through the API. |
| `status` | any time | Prints the recorded step and last error of every deployment. |

Every phase skips what an earlier run already finished, so a phase that failed for some deployments is run again as is. `--deployments 12,34` limits a phase to those deployments, which is how a canary goes first.

The command reads the instance manager's own environment: `DATABASE_*`, `INSTANCE_PARAMETER_ENCRYPTION_KEY`, `S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `SOPS_*`, `HOSTNAME` as the API url, and `ADMIN_USER_EMAIL`/`ADMIN_USER_PASSWORD` for `deploy` and `cleanup`.

## Order

```sh
migrate-v3 plan                         # days ahead, and again right before the window
# stop the instance manager (the window starts)
migrate-v3 migrate                      # refuses to run while an instance manager answers at HOSTNAME
# deploy version 3.0 (the window ends)
migrate-v3 deploy --deployments <canary>
migrate-v3 deploy
migrate-v3 cleanup                      # after the grace period
```

## Running it

### im-vm

The binary ships in the image as `/app/migrate-v3`. Run it on the environment's compose network with the environment file, the AWS credentials the host mints and the image tag of version 3.0:

```sh
. /opt/im/server.conf
sudo docker run --rm --network im-dev_default \
  --env-file /opt/im/environments/dev.env \
  --volume /opt/im/credentials:/aws:ro --env AWS_CONFIG_FILE=/aws/config --env AWS_REGION="$AWS_REGION" \
  --entrypoint /app/migrate-v3 dhis2/im-manager:<3.0 tag> plan
```

`docker compose --project-name im-dev stop api` opens the window and `im-environment up dev <3.0 tag>` closes it.

### EKS

Scale `deployment/im-manager-prod` to 0 for the window and run the same image as a pod with the deployment's `envFrom` and service account (IRSA grants the S3 and KMS access):

```sh
kubectl --namespace instance-manager-prod get deployment im-manager-prod --output json \
  | jq '.spec.template | .spec.restartPolicy = "Never" | .spec.containers[0].command = ["/app/migrate-v3", "migrate"]
        | {apiVersion: "v1", kind: "Pod", metadata: {name: "migrate-v3"}, spec: .spec}' \
  | kubectl --namespace instance-manager-prod apply --filename -
```

## Rehearsal

1. A `feat-*` environment running the last release before 3.0, with a few deployments holding data, files and a pgAdmin, some paused.
2. `plan`, `migrate`, start 3.0, `deploy`, then check the data, the files and pgAdmin on every deployment, and that a reset seeds from the original database.
3. Dev, which also measures the dump and restore throughput the production window is estimated from.
