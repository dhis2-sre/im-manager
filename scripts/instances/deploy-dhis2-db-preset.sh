#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2-db

GROUP=$1
NAME=$2

DATABASE_ID=${DATABASE_ID:-1}
DATABASE_SIZE=${DATABASE_SIZE:-10Gi}
INSTANCE_TTL=${INSTANCE_TTL:-0}

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
    }
 ]
}" | $HTTP post "$IM_HOST/instances?preset=true" "Authorization: Bearer $ACCESS_TOKEN"
