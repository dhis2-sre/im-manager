#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=whoami-go

GROUP=$1
NAME=$2
PRESET=$3

PRESET_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP/$PRESET" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"presetInstance\": $PRESET_ID
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
