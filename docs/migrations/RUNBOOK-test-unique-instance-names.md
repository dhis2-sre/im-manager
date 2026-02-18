# Runbook: Test unique-instance-names migration (copy-paste)

**Goal:** 5 DHIS2 (dhis2-core + dhis2-db) + 5 DHIS2+PgAdmin (dhis2-core + dhis2-db + pgadmin) on **master**, back up, switch IM to **feat/unique-instance-names**, then run migration (dry-run then optionally apply).

**Set these env variables once:**

```bash
export GROUP=whoami
export IM_HOST=https://your-IM.example.com
export USER_EMAIL=your@email
export PASSWORD=yourpassword
export HTTP="http --verify=no --check-status"
```

---

## Phase 1: IM on **master** — create 10 deployments

From repo root:

```bash
cd scripts/instances
```

**5 DHIS2-only deployments:**

```bash
for i in {1..5}; do
  ALLOW_SUSPEND=false ./deploy-dhis2.sh "$GROUP" "migrate-test-dhis2-$i" "Migration test DHIS2 $i"
done
```

**5 DHIS2+PgAdmin deployments (create DHIS2 first, then add PgAdmin and redeploy):**

```bash
for i in {1..5}; do
  ALLOW_SUSPEND=false ./deploy-dhis2.sh "$GROUP" "migrate-test-pgadmin-$i" "Migration test PgAdmin $i"
done
```

**Add PgAdmin instance and redeploy for each of the 5:**

```bash
./findDeployments.sh > .deployments.json
for i in {1..5}; do
  name="migrate-test-pgadmin-$i"
  DEP_ID=$(jq -r ".[] | select(.name==\"$GROUP\") | .deployments[] | select(.name==\"$name\") | .id" .deployments.json)
  ./create-deployment-instance-pgadmin.sh "$DEP_ID"
  ./deploy-deployment.sh "$DEP_ID"
done
```

Wait until all 10 deployments are running (DHIS2 and DB up; PgAdmin up for the 5 pgadmin ones).

---

## Phase 2: Backup (still on **master**)

Still from `scripts/instances/` dir:

```bash
python3 backup_group_dbs.py \
  --group "$GROUP" \
  --out ./$GROUP-db-backups.json \
  --host "$IM_HOST" \
  --token "$ACCESS_TOKEN"
```

Check the file:

```bash
jq '.items | length' ./$GROUP-db-backups.json
```

Should be 10. Keep `./$GROUP-db-backups.json` for the next phases.

---

## Phase 3: Deploy **feat/unique-instance-names** on IM

**Do this in your cluster/CI:** switch IM to the `feat/unique-instance-names` branch and deploy so the IM restarts. The `groups` table will get an `id` column via AutoMigrate.

---

## Phase 4: Port-forward to IM Postgres

In a **dedicated terminal** (leave it open):

```bash
kubectl port-forward -n <IM_NAMESPACE> svc/<IM_POSTGRES_SVC> 5432:5432
```

Replace `<IM_NAMESPACE>` and `<IM_POSTGRES_SVC>` with your IM Postgres namespace and service name.

---

## Phase 5: Run migration (dry-run)

In **another terminal**, from **repo root**:

```bash
export DATABASE_HOST=127.0.0.1
export DATABASE_PORT=5432
export DATABASE_USERNAME=<IM_DB_USER>
export DATABASE_PASSWORD=<IM_DB_PASSWORD>
export DATABASE_NAME=<IM_DB_NAME>

go run ./cmd/migrate-unique-instance-names \
  -json-file ./$GROUP-db-backups.json \
  -dry-run
```

Replace `<IM_DB_USER>`, `<IM_DB_PASSWORD>`, `<IM_DB_NAME>` with the IM Postgres credentials.

**Expected:** Exit 0, logs like "validation passed", "would set instance id=…", "dry run: rolled back (no changes made)".

---

## Phase 6 (optional): Apply migration and verify

Same terminal (port-forward still running), **without** `-dry-run`:

```bash
go run ./cmd/migrate-unique-instance-names -json-file ./$GROUP-db-backups.json
```

Then **reset** all deployments in the group so instances are recreated with the new pod names. From `scripts/instances` (with `$GROUP` and auth set):

```bash
cd scripts/instances
./reset-group-deployments.sh "$GROUP"
```

Or inline (reuse `.deployments.json` if you already have it, or run `./findDeployments.sh > .deployments.json` first):

```bash
cd scripts/instances
./findDeployments.sh > .deployments.json
for inst_id in $(jq -r ".[] | select(.name==\"$GROUP\") | .deployments[] | .instances[]? | .id" .deployments.json); do
  ./reset.sh "$inst_id"
done
```

Confirm instances come up and DHIS2/PgAdmin can reach the DB (hostname format `{name}-{group.id}-database-postgresql.{namespace}.svc`).

---

## Phase 7: Restore DATABASE_ID to original values

After reset, set each dhis2-db instance's DATABASE_ID parameter back to the original (pre–save-as) database id so the deployment uses the original database. The backup JSON must include `originalDatabaseId` per item (produced by the backup script in Phase 2).

**Prerequisite:** Same backup JSON used for the migration (e.g. `./$GROUP-db-backups.json` from `scripts/instances`), port-forward to IM Postgres still running (same as Phase 5/6), and `DATABASE_*` env set.

From **repo root** (same terminal as Phase 5/6), with the backup JSON path matching where you ran the migration (e.g. `./$GROUP-db-backups.json` if the file is in the current directory, or `./scripts/instances/$GROUP-db-backups.json` if it is in scripts/instances):

```bash
# Optional: dry-run first
go run ./cmd/restore-database-ids -json-file ./$GROUP-db-backups.json -dry-run

# Apply
go run ./cmd/restore-database-ids -json-file ./$GROUP-db-backups.json
```
