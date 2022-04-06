#!/usr/bin/env bash

set -euo pipefail

if [ -n "$DATABASE_ID" ]; then
#  ABSOLUTE_SEED_URL="$DATABASE_MANAGER_SERVICE_HOST/$DATABASE_MANAGER_SERVICE_BASE_PATH/databases/$DATABASE_ID/download"
  ABSOLUTE_SEED_URL="im-database-manager-dev.instance-manager-dev.svc:8080/databases/$DATABASE_ID/download"
  echo "DATABASE_HOST: $ABSOLUTE_SEED_URL"

  MY_UID=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 32 ; echo '')
  BASE_FOLDER="$POSTGRESQL_VOLUME_DIR/im"
  mkdir -p $BASE_FOLDER
  DOWNLOAD_FILE="$BASE_FOLDER/$MY_UID"
  DATA_FILE="$DOWNLOAD_FILE-seed-data"

  curl --fail -H "Authorization: $IM_ACCESS_TOKEN" -L "$ABSOLUTE_SEED_URL" -o "$DOWNLOAD_FILE" | cat
  echo "Download completed!"

  if gunzip -t "$DOWNLOAD_FILE"; then
    echo "Unzipping..."
    gunzip -v -c "$DOWNLOAD_FILE" > "$DATA_FILE"
    echo "Unzipping completed!"
  else
    echo "No unzip!"
    DATA_FILE="$DOWNLOAD_FILE"
  fi

  # file (the unix util) isn't available on bitnami's postgresql image therefore the following hack is used
  # If the first line of the seed file is "--" it's assumed it's sql and not pgc
  firstLine=$(head -n 1 "$DATA_FILE")
  if [ "$firstLine" == "--" ]; then
    psql -U postgres -d "$DATABASE_NAME" -f "$DATA_FILE"
  else
    pg_restore -j 8 -U postgres -d "$DATABASE_NAME" "$DATA_FILE"
  fi

  rm -f "$DOWNLOAD_FILE" "$DATA_FILE"

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
