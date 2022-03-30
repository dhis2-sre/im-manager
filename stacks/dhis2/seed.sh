#!/usr/bin/env bash

set -euo pipefail

if [ -n "$DATABASE_ID" ]; then
#  ABSOLUTE_SEED_URL="https://databases.dhis2.org/sierra-leone/$SEED_URL"
#  ABSOLUTE_SEED_URL="$DATABASE_MANAGER_SERVICE_HOST/$DATABASE_MANAGER_SERVICE_BASE_PATH/databases/$DATABASE_ID/download"
  ABSOLUTE_SEED_URL="im-database-manager-dev.instance-manager-dev.svc:8080/databases/$DATABASE_ID/download"
  echo "DATABASE_HOST: $ABSOLUTE_SEED_URL"
  DATA_FOLDER="$POSTGRESQL_VOLUME_DIR/t$$"
  curl --fail -H "Authorization: $IM_ACCESS_TOKEN" -L "$ABSOLUTE_SEED_URL" -o "$DATA_FOLDER" | cat
  gunzip -v -c "$DATA_FOLDER" > "$DATA_FOLDER-seed-data"
  # file (the unix util) isn't available on bitnami's postgresql image therefore the following hack is used
  # If the first line of the seed file is "--" it's assumed it's sql and not pgc
  firstLine=$(head -n 1 "$DATA_FOLDER-seed-data")
  if [ "$firstLine" == "--" ]; then
    psql -U postgres -d "$DATABASE_NAME" -f "$DATA_FOLDER-seed-data"
  else
    pg_restore -j 8 -U postgres -d "$DATABASE_USERNAME" "$DATA_FOLDER-seed-data"
  fi
  rm "$DATA_FOLDER" "$DATA_FOLDER-seed-data"

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
  psql -U postgres -d "$DATABASE_USERNAME" -p 5432 -c "create extension if not exists postgis"
fi
