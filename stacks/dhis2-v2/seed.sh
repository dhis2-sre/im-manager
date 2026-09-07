# Runs inside the dhis2 chart's seed job, which prepares superuser PGHOST/PGUSER/PGPASSWORD/
# PGDATABASE, waits for the database and guards with a marker table; DHIS 2 waits for the marker.

function exec_psql() {
  psql --no-align --tuples-only --command="$1"
}

if [[ -z "${DATABASE_DOWNLOAD_URL:-}" ]]; then
  echo "Seeding aborted. No database download URL found!"
  exit 0
fi

echo "DATABASE_DOWNLOAD_URL: $DATABASE_DOWNLOAD_URL"

exec_psql "create extension if not exists postgis"
exec_psql "create extension if not exists pg_trgm"
exec_psql "create extension if not exists btree_gin"

tmp_file=$(mktemp)
curl --connect-timeout 10 --retry 5 --retry-delay 1 --fail -L "$DATABASE_DOWNLOAD_URL" >"$tmp_file" || {
  echo "curl failed with exit code $?"
  exit 1
}

# Try pg_restore... Or gzipped sql
# pg_restore often returns a non zero return code due to benign errors resulting in executing of gunzip despite the restore being successful
# gunzip will fail because the input isn't gzipped causing the whole seed script to fail... Which is why there's a "|| true" at the end
(pg_restore --verbose -d "$PGDATABASE" -j 4 "$tmp_file") ||
  (gunzip -v -c "$tmp_file" | psql) || true
rm "$tmp_file"

## Change ownership to $DATABASE_USERNAME
change_owner() {
  local query=$1
  local obj_type=$2

  entities=$(exec_psql "$query")
  for entity in $entities; do
    echo "Changing owner of $obj_type $entity to $DATABASE_USERNAME"
    exec_psql "ALTER $obj_type \"$entity\" OWNER TO $DATABASE_USERNAME"
  done
}

change_owner_routines() {
  local query="
    SELECT format(
             'ALTER %s %s OWNER TO %I',
             CASE p.prokind
               WHEN 'p' THEN 'PROCEDURE'
               WHEN 'a' THEN 'AGGREGATE'
               ELSE 'FUNCTION'
             END,
             p.oid::regprocedure,
             '$DATABASE_USERNAME'
           )
    FROM pg_proc p
    JOIN pg_namespace n ON n.oid = p.pronamespace
    WHERE n.nspname = 'public'
      AND p.prokind IN ('f', 'p', 'a')
      AND NOT EXISTS (
        SELECT 1
        FROM pg_depend d
        WHERE d.classid = 'pg_proc'::regclass
          AND d.objid = p.oid
          AND d.deptype = 'e'
      )
  "

  local statements
  statements=$(exec_psql "$query")

  while IFS= read -r statement; do
    [[ -z "$statement" ]] && continue
    echo "$statement"
    exec_psql "$statement"
  done <<<"$statements"
}

change_owner "SELECT tablename FROM pg_tables WHERE schemaname = 'public'" "TABLE"
change_owner "SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = 'public'" "SEQUENCE"
change_owner "SELECT table_name FROM information_schema.views WHERE table_schema = 'public'" "VIEW"
change_owner_routines
