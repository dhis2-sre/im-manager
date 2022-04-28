#!/usr/bin/env bash

set -euo pipefail

if [ -n "$DATABASE_ID" ]; then
  DATABASE_MANAGER_HOSTNAME="im-database-manager-tons.instance-manager-tons.svc:8080/databases/$DATABASE_ID/download"
  echo "DATABASE_HOST: $DATABASE_MANAGER_HOSTNAME"

# Try pg_restore... Or gzipped sql
  (curl --fail -L "$DATABASE_MANAGER_HOSTNAME" -H "Authorization: $IM_ACCESS_TOKEN" | pg_restore -U postgres -d "$DATABASE_NAME") || \
  (curl --fail -L "$DATABASE_MANAGER_HOSTNAME" -H "Authorization: $IM_ACCESS_TOKEN" | gunzip -v -c | psql -U postgres -d "$DATABASE_NAME")

  ## Change ownership to $DATABASE_USERNAME
  # Tables
  entities=$(psql -U postgres -qAt -c "select tablename from pg_tables where schemaname = 'public'" "$DATABASE_NAME")
  for entity in $entities; do
    echo "Changing owner of $entity to $DATABASE_USERNAME"
    psql -U postgres -c "alter table \"$entity\" owner to $DATABASE_USERNAME" "$DATABASE_NAME"
  done

  # Sequences
  entities=$(psql -U postgres -qAt -c "select sequence_name from information_schema.sequences where sequence_schema = 'public'" "$DATABASE_NAME")
  for entity in $entities; do
    echo "Changing owner of $entity to $DATABASE_USERNAME"
    psql -U postgres -c "alter sequence \"$entity\" owner to $DATABASE_USERNAME" "$DATABASE_NAME"
  done

  # Views
  entities=$(psql -U postgres -qAt -c "select table_name from information_schema.views where table_schema = 'public'" "$DATABASE_NAME")
  for entity in $entities; do
    echo "Changing owner of $entity to $DATABASE_USERNAME"
    psql -U postgres -c "alter view \"$entity\" owner to $DATABASE_USERNAME" "$DATABASE_NAME"
  done

else
  psql -U postgres -d "$DATABASE_USERNAME" -c "create extension if not exists postgis"
fi
