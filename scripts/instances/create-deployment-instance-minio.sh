#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=minio

DATABASE_ID=${DATABASE_ID:-whoami-2-42-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-20Gi}

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"parameters\": {
    \"DATABASE_ID\": {
      \"value\": \"$DATABASE_ID\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
