# backfill-companion-minio

One-time migration that adds missing `minio` deployment instances for old deployments created before companion stacks were introduced. Deployments that have `dhis2-core` with `STORAGE_TYPE=minio` but no separate `minio` instance row fail on TTL destroy with `"minio" is required by "dhis2-core"`. This command inserts the missing minio instance (and parameters) so destroy can succeed.

The migration is idempotent - only inserts when a minio instance is missing; re-run is a no-op.

## Prerequisites

First port-forward the Kubernetes database so backup and migration commands can reach it (run in a separate terminal and leave it open):

```bash
kubectl port-forward svc/im-manager-postgresql 5432:5432 -n instance-manager-feature
```

Export the required database environment variables:

```bash
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USERNAME=instance-manager
export DATABASE_PASSWORD=secret
export DATABASE_NAME=instance-manager
```

## Dry run

Pass `--dry-run/-n` to print what would be changed without writing to the DB:

```bash
go run ./cmd/backfill-companion-minio --dry-run
```

## Testing the migration (before production)

1. **Create old state:** Deploy an **old** version of the app (`v0.63.0`, before the companions stack change) and create a deployment.
2. **Upgrade:** Deploy **master**. Trigger TTL destroy or manually delete the deployment → expect error `"minio" is required by "dhis2-core"`.
3. **Dry run:** `go run ./cmd/backfill-companion-minio --dry-run` with DB env → verify it lists the deployment(s) that would get a minio instance.
4. **Run backfill:** `go run ./cmd/backfill-companion-minio` with DB env. Verify the deployment now has an extra instance row for stack `minio` with the same `(name, group_name)` as the dhis2-core instance.
5. **Deploy the deployment:** Trigger a deploy so the new minio instance is deployed to the cluster (e.g. `./scripts/instances/deploy-deployment.sh <deployment-id>`). Existing pods will restart; PVCs are preserved.
6. **Verify destroy:** Trigger destroy again → should succeed. Optionally confirm in the cluster that the minio release and PVCs are gone.
7. **Idempotency:** Run the backfill again → exit 0, no new rows.

## Backup (production)

Before running the migration on production, take a logical backup of the database. `pg_dump` works against a running database and does not require downtime. Uses the same env vars from [Prerequisites](#prerequisites):

```bash
PGPASSWORD="$DATABASE_PASSWORD" pg_dump -h "$DATABASE_HOST" -p "$DATABASE_PORT" -U "$DATABASE_USERNAME" -d "$DATABASE_NAME" -Fp -f "${ENVIRONMENT}_im_manager_backfill_pre_$(date +%Y%m%d_%H%M%S).sql"
```

To restore if needed (use the path of the backup file you created; use only when restoring over the same database):

```bash
PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DATABASE_HOST" -p "$DATABASE_PORT" -U "$DATABASE_USERNAME" -d "$DATABASE_NAME" -f "${ENVIRONMENT}_im_manager_backfill_pre_YYYYMMDD_HHMMSS.sql"
```

## Migration steps (production)

0. **Backup** the database (see [Backup (production)](#backup-production) above).
1. **Dry run:** `go run ./cmd/backfill-companion-minio --dry-run` — review which deployments (by group and name) would be updated. The output ends with **Deployment IDs that would need deploy (after backfill):** and a space-separated list of IDs.
2. **Run backfill.** Inserts the missing minio instance rows and parameters. Save the output so you can read the deployment IDs and run the deploy step later:

   ```bash
   go run ./cmd/backfill-companion-minio 2>&1 | tee backfill.out
   ```

   Review the output; the last line is **Deployment IDs to deploy:** followed by a space-separated list of IDs.
3. **Deploy each affected deployment** so the new minio instance is deployed to the cluster. Run this when ready (e.g. after reviewing the backfill output). Existing pods restart; PVCs are preserved:

   ```bash
   ids=$(grep "Deployment IDs to deploy:" backfill.out | sed 's/.*: *//; s/"$//')
   cd scripts/instances
   for id in $ids; echo "Deploying $id"; do ./deploy-deployment.sh "$id"; done
   ```
