#!/usr/bin/env bash

set -euo pipefail

psql -U postgres -qAt -d "$DATABASE_NAME" -c "create extension if not exists postgis"

if [ -n "$DATABASE_ID" ]; then
  table_exists=$(psql -U postgres -qAt -c "select exists (select from information_schema.tables where table_schema = 'schema_name' and table_name = 'table_name')" "$DATABASE_NAME")
  if [ "$table_exists" = "true" ]; then
    row_count=$(psql -U postgres -qAt -c "select count(*) from organisationunit" "$DATABASE_NAME")
    if [ $row_count -ne 0 ]; then
      echo "Seeding aborted!"
      exit 0
    fi
  fi

  DATABASE_MANAGER_ABSOLUTE_URL="$DATABASE_MANAGER_URL:8080/databases/$DATABASE_ID/download"
  echo "DATABASE_MANAGER_ABSOLUTE_URL: $DATABASE_MANAGER_ABSOLUTE_URL"

# Try pg_restore... Or gzipped sql
  (curl --fail -L "$DATABASE_MANAGER_ABSOLUTE_URL" -H "Authorization: $IM_ACCESS_TOKEN" | pg_restore -U postgres -d "$DATABASE_NAME") || \
  (curl --fail -L "$DATABASE_MANAGER_ABSOLUTE_URL" -H "Authorization: $IM_ACCESS_TOKEN" | gunzip -v -c | psql -U postgres -d "$DATABASE_NAME")

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

fi
