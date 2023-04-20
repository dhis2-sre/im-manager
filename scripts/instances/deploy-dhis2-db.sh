#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2-db

GROUP=$1
NAME=$2

DATABASE_ID=${DATABASE_ID:-1}
DATABASE_SIZE=${DATABASE_SIZE:-5Gi}
INSTANCE_TTL=${INSTANCE_TTL:-""}
RESOURCES_REQUESTS_CPU=${RESOURCES_REQUESTS_CPU:-250m}
RESOURCES_REQUESTS_MEMORY=${RESOURCES_REQUESTS_MEMORY:-256Mi}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"requiredParameters\": [
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ],
  \"optionalParameters\": [
    {
      \"name\": \"INSTANCE_TTL\",
      \"value\": \"$INSTANCE_TTL\"
    },
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
