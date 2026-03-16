#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2

DEPLOYMENT_ID=$(./findByName.sh "$GROUP" "$NAME" | jq -r '.id')

INSTANCE_ID=$($HTTP get "$IM_HOST/deployments/$DEPLOYMENT_ID" "Authorization: Bearer $ACCESS_TOKEN" | jq -r '.instances[] | select(.stackName=="dhis2-db") | .id')

DATABASE_ID=${DATABASE_ID:-test-dbs-sierra-leone-2-40-2-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-20Gi}
DB_RESOURCES_REQUESTS_CPU=${DB_RESOURCES_REQUESTS_CPU:-250m}
DB_RESOURCES_REQUESTS_MEMORY=${DB_RESOURCES_REQUESTS_MEMORY:-256Mi}

echo "{
  \"stackName\": \"dhis2-db\",
  \"parameters\": {
    \"DATABASE_ID\": {
      \"value\": \"$DATABASE_ID\"
    },
    \"DATABASE_SIZE\": {
      \"value\": \"$DATABASE_SIZE\"
    },
    \"RESOURCES_REQUESTS_CPU\": {
      \"value\": \"$DB_RESOURCES_REQUESTS_CPU\"
    },
    \"RESOURCES_REQUESTS_MEMORY\": {
      \"value\": \"$DB_RESOURCES_REQUESTS_MEMORY\"
    }
  }
}" | $HTTP put "$IM_HOST/deployments/$DEPLOYMENT_ID/instance/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
