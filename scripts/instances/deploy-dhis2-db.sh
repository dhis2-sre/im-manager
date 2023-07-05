#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2-db

GROUP=$1
NAME=$2

DATABASE_ID=${DATABASE_ID:-whoami-sierra-leone-2-40-0-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-5Gi}
INSTANCE_TTL=${INSTANCE_TTL:-0}
RESOURCES_REQUESTS_CPU=${RESOURCES_REQUESTS_CPU:-250m}
RESOURCES_REQUESTS_MEMORY=${RESOURCES_REQUESTS_MEMORY:-256Mi}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"ttl\": $INSTANCE_TTL,
  \"requiredParameters\": [
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ],
  \"optionalParameters\": [
    {
      \"name\": \"DATABASE_SIZE\",
      \"value\": \"$DATABASE_SIZE\"
    },
    {
      \"name\": \"RESOURCES_REQUESTS_CPU\",
      \"value\": \"$RESOURCES_REQUESTS_CPU\"
    },
    {
      \"name\": \"RESOURCES_REQUESTS_MEMORY\",
      \"value\": \"$RESOURCES_REQUESTS_MEMORY\"
    }
 ]
}" | $HTTP post "$IM_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
