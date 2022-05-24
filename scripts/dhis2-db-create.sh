#!/usr/bin/env bash

set -euo pipefail

STACK_NAME=dhis2-db

NAME=$1
GROUP_NAME=$2

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP_NAME\",
  \"stackName\": \"$STACK_NAME\"
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
