#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=whoami-go

GROUP=$1
NAME=$2

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\"
}" | $HTTP post "$INSTANCE_HOST/instances?deploy=false" "Authorization: Bearer $ACCESS_TOKEN"
