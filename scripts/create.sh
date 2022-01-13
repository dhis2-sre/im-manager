#!/usr/bin/env bash

set -e

HTTP="http --verify=no --check-status"

STACK_ID=2

NAME=$1
GROUP_NAME=$2

GROUP_ID=$($HTTP --check-status "$INSTANCE_HOST/groups-name-to-id/$GROUP_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$NAME\",
  \"groupId\": $GROUP_ID,
  \"stackId\": $STACK_ID
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
