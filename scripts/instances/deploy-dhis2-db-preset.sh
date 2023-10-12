#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2-db

GROUP=$1
NAME=$2

DATABASE_ID=${DATABASE_ID:-whoami-sierra-leone-2-40-0-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-20Gi}
INSTANCE_TTL=${INSTANCE_TTL:-0}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"ttl\": $INSTANCE_TTL,
  \"parameters\": [
    {
      \"name\": \"DATABASE_SIZE\",
      \"value\": \"$DATABASE_SIZE\"
    },
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
 ]
}" | $HTTP post "$IM_HOST/instances?preset=true" "Authorization: Bearer $ACCESS_TOKEN"
