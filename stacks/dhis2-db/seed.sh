#!/usr/bin/env bash

set -euo pipefail

export PGPASSWORD=$POSTGRES_POSTGRES_PASSWORD

function exec_psql() {
  psql -U postgres -qAt -d "$DATABASE_NAME" -c "$1"
}

if [[ -z $DATABASE_ID ]]; then
  echo "Seeding aborted. No database id found!"
  exit 0
fi

DATABASE_DOWNLOAD_URL="$HOSTNAME/databases/$DATABASE_ID/download"
echo "DATABASE_DOWNLOAD_URL: $DATABASE_DOWNLOAD_URL"

exec_psql "create extension if not exists postgis"
exec_psql "create extension if not exists pg_trgm"
exec_psql "create extension if not exists btree_gin"

tmp_file=$(mktemp)
curl --connect-timeout 10 --retry 5 --retry-delay 1 --fail -L "$DATABASE_DOWNLOAD_URL" --cookie "accessToken=$IM_ACCESS_TOKEN" >"$tmp_file"

# Try pg_restore... Or gzipped sql
# pg_restore often returns a non zero return code due to benign errors resulting in executing of gunzip despite the restore being successful
# gunzip will fail because the input isn't gzipped causing the whole seed script to fail... Which is why there's a "|| true" at the end
(pg_restore --verbose -U postgres -d "$DATABASE_NAME" -j 4 "$tmp_file") ||
  (gunzip -v -c "$tmp_file" | psql -U postgres -d "$DATABASE_NAME") || true
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

change_owner "SELECT tablename FROM pg_tables WHERE schemaname = 'public'" "TABLE"
change_owner "SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = 'public'" "SEQUENCE"
change_owner "SELECT table_name FROM information_schema.views WHERE table_schema = 'public'" "VIEW"
