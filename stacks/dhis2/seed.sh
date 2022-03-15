#!/usr/bin/env bash

set -euo pipefail

if [ -n "$DATABASE_ID" ]; then
#  ABSOLUTE_SEED_URL="https://databases.dhis2.org/sierra-leone/$SEED_URL"
#  ABSOLUTE_SEED_URL="$DATABASE_MANAGER_SERVICE_HOST/$DATABASE_MANAGER_SERVICE_BASE_PATH/databases/$DATABASE_ID/download"
  ABSOLUTE_SEED_URL="im-database-manager-dev.instance-manager-dev.svc:8080/databases/$DATABASE_ID/download"
  echo "DATABASE_HOST: $ABSOLUTE_SEED_URL"
  curl --fail -H "Authorization: $IM_ACCESS_TOKEN" -L "$ABSOLUTE_SEED_URL" -o /tmp/t$$ | cat
  gunzip -c /tmp/t$$ > /tmp/t$$-seed-data
  # file (the unix util) isn't available on bitnami's postgresql image therefore the following hack is used
  # If the first line of the seed file is "--" it's assumed it's sql and not pgc
  firstLine=$(head -n 1 /tmp/t$$-seed-data)
  if [ "$firstLine" == "--" ]; then
    psql -U postgres -d "$DATABASE_NAME" -f /tmp/t$$-seed-data
    tables=$(psql -U postgres -qAt -c "select tablename from pg_tables where schemaname = 'public'" "$DATABASE_NAME")
    for table in $tables; do
      echo "Changing owner of $table to $DATABASE_USERNAME"
      psql -U postgres -c "alter table \"$table\" owner to $DATABASE_USERNAME" "$DATABASE_NAME"
    done
  else
    pg_restore -j 8 -U postgres -d "$DATABASE_USERNAME" /tmp/t$$-seed-data
  fi
  rm /tmp/t$$ /tmp/t$$-seed-data
else
  psql -U postgres -d "$DATABASE_USERNAME" -p 5432 -c "create extension if not exists postgis"
fi
