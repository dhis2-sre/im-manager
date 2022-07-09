#!/usr/bin/env bash

set -euo pipefail

DATABASE_ID=1

STACK_NAME=dhis2-db

INSTANCE_NAME=$1
GROUP_NAME=$2

INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$INSTANCE_NAME\",
  \"groupName\": \"$GROUP_NAME\",
  \"stackName\": \"$STACK_NAME\",
  \"requiredParameters\": [
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
