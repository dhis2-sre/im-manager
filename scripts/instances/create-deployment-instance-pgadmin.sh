#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=pgadmin

PGADMIN_USERNAME="pgadmin-user@dhis2.org"
echo "PgAdmin username: $PGADMIN_USERNAME"
PGADMIN_PASSWORD=$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c32)
echo "PgAdmin password: $PGADMIN_PASSWORD"

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"parameters\": {
    \"PGADMIN_USERNAME\": {
      \"value\": \"$PGADMIN_USERNAME\"
    },
    \"PGADMIN_PASSWORD\": {
      \"value\": \"$PGADMIN_PASSWORD\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
