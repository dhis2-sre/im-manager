#!/usr/bin/env bash

set -euo pipefail

STACK_NAME=dhis2-core

NAME=$1
GROUP_NAME=$2

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP_NAME\",
  \"stackName\": \"$STACK_NAME\"
}" | $HTTP post "$INSTANCE_HOST/instances?deploy=false" "Authorization: Bearer $ACCESS_TOKEN"
