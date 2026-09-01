#!/usr/bin/env bash

set -euo pipefail

export PGPASSWORD=$POSTGRES_POSTGRES_PASSWORD

function exec_psql() {
  psql --username=postgres --no-align --tuples-only --dbname="$DATABASE_NAME" --command="$1"
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

if pg_restore --list "$tmp_file" >/dev/null 2>&1; then
  echo "Restoring pg_dump archive"
  # pg_restore exits non zero on benign errors such as a missing role, and --list has already
  # rejected a corrupt archive, so its exit code is logged rather than treated as fatal.
  pg_restore --verbose -U postgres -d "$DATABASE_NAME" -j 4 "$tmp_file" || echo "pg_restore exited with $?, continuing"
elif gunzip --test "$tmp_file" 2>/dev/null; then
  # A truncated dump still decompresses cleanly, so pg_dump's completion marker is the only proof it
  # is whole, and only pg_dump writes it. head and tail exit before gunzip finishes, hence pipefail
  # is disabled for these two subshells.
  dump_head=$(set +o pipefail; gunzip -c "$tmp_file" 2>/dev/null | head -c 4096)
  if [[ "$dump_head" == *"PostgreSQL database dump"* ]]; then
    dump_tail=$(set +o pipefail; gunzip -c "$tmp_file" 2>/dev/null | tail -c 512)
    if [[ "$dump_tail" != *"PostgreSQL database dump complete"* ]]; then
      echo "Refusing to restore: pg_dump output is truncated, it does not end with the completion marker"
      exit 1
    fi
  fi

  echo "Restoring gzipped SQL"
  gunzip -c "$tmp_file" | psql --username=postgres --dbname="$DATABASE_NAME"
else
  echo "Not restoring: the download is neither a pg_dump archive nor gzipped SQL"
fi
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
