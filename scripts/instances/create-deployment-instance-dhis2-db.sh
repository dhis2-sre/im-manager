#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=dhis2-db

DATABASE_ID=${DATABASE_ID:-whoami-dhis2-db-sierra-leone-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-20Gi}

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"parameters\": {
    \"DATABASE_ID\": {
      \"value\": \"$DATABASE_ID\"
    },
    \"DATABASE_SIZE\": {
      \"value\": \"$DATABASE_SIZE\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
