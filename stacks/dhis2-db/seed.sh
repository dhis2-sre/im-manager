#!/usr/bin/env bash

set -euo pipefail

if [ -n "$DATABASE_ID" ]; then
#  ABSOLUTE_SEED_URL="https://databases.dhis2.org/sierra-leone/$SEED_URL"
  ABSOLUTE_SEED_URL="$DATABASE_MANAGER_SERVICE_HOST/$DATABASE_MANAGER_SERVICE_BASE_PATH/databases/$DATABASE_ID/download"
  curl -H "Authorization: $ACCESS_TOKEN" -L "$ABSOLUTE_SEED_URL" -o /tmp/t$$ | cat
  gunzip -c /tmp/t$$ > /tmp/t$$-seed-data
  firstLine=$(head -n 1 /tmp/t$$-seed-data)
  # file (the unix util) isn't available on in bitnami's postgresql image therefore the following hack is used
  # If the first line of the seed file is "--" it's assumed it's sql and not pgc
  if [ "$firstLine" == "--" ]; then
    psql -U postgres -d dhis2 -p 5432 -f /tmp/t$$-seed-data
  else
    pg_restore -j 8 -U postgres -d dhis2 /tmp/t$$-seed-data
  fi
  rm /tmp/t$$ /tmp/t$$-seed-data
else
  psql -U postgres -d dhis2 -p 5432 -c "create extension if not exists postgis;"
fi
