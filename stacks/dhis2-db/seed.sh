#!/usr/bin/env bash

set -euo pipefail

function exec_psql() {
  PGPASSWORD=$POSTGRES_POSTGRES_PASSWORD psql -U postgres -qAt -d "$DATABASE_NAME" -c "$1"
}

if [[ -z $DATABASE_ID ]]; then
  echo "Seeding aborted. No database id found!"
  exit 0
fi

DATABASE_MANAGER_ABSOLUTE_URL="$DATABASE_MANAGER_URL/databases/$DATABASE_ID/download"
echo "DATABASE_MANAGER_ABSOLUTE_URL: $DATABASE_MANAGER_ABSOLUTE_URL"

exec_psql "create extension if not exists postgis"
exec_psql "create extension if not exists pg_trgm"
exec_psql "create extension if not exists btree_gin"

tmp_file=$(mktemp)
curl --connect-timeout 10 --retry 5 --retry-delay 1 --fail -L "$DATABASE_MANAGER_ABSOLUTE_URL" -H "Authorization: $IM_ACCESS_TOKEN" >"$tmp_file"
# Try pg_restore... Or gzipped sql
# pg_restore often returns a non zero return code due to benign errors resulting in executing of gunzip despite the restore being successful
# gunzip will fail because the input isn't gzipped causing the whole seed script to fail... Which is why there's a "|| true" at the end
(pg_restore --verbose -U postgres -d "$DATABASE_NAME" -j 4 "$tmp_file") ||
  (gunzip -v -c "$tmp_file" | psql -U postgres -d "$DATABASE_NAME") || true
rm "$tmp_file"

## Change ownership to $DATABASE_USERNAME
exec_psql "grant all privileges on all tables in schema public to $DATABASE_USERNAME"
exec_psql "grant all privileges on all sequences in schema public to $DATABASE_USERNAME"
# At some point we need to grant access to view while deploying using IM, I'm leaving the below here as an easy fix in case the problem shows up here
#psql -At -d dhis2 -c "SELECT 'GRANT ALL ON '||viewname||' TO $DATABASE_USERNAME;' FROM pg_views WHERE schemaname='public';" | psql -d dhis2
