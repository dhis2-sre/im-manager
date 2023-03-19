#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2

GROUP=$1
NAME=$2
PRESET=$3

PRESET_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$PRESET" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"presetInstance\": $PRESET_ID
}" | $HTTP post "$IM_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
