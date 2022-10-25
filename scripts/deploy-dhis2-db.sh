#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2-db

GROUP=$1
NAME=$2

DATABASE_ID=${DATABASE_ID:-1}
DATABASE_SIZE=${DATABASE_SIZE:-5Gi}
INSTANCE_TTL=${INSTANCE_TTL:-""}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"optionalParameters\": [
    {
      \"name\": \"INSTANCE_TTL\",
      \"value\": \"$INSTANCE_TTL\"
    },
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    },
    {
      \"name\": \"DATABASE_SIZE\",
      \"value\": \"$DATABASE_SIZE\"
    }
 ]
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
