#!/usr/bin/env bash

set -euxo pipefail

function int_handler {
  $HTTP delete "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
  trap - EXIT
  exit
}

trap int_handler INT
trap int_handler EXIT

INSTANCE_OUTPUT=$(echo "{
  \"name\": \"$1\",
  \"groupId\": 2,
  \"stackId\": 2
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN")

#echo "$INSTANCE_OUTPUT" | jq

INSTANCE_ID=$(echo "$INSTANCE_OUTPUT" | jq -r '.ID')

$HTTP "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"

INSTANCE_LAUNCH_OUTPUT=$(echo "{
  \"requiredParameters\": [
    {
      \"stackParameterId\": 1,
      \"value\": \"0.5.0\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN")

echo "$INSTANCE_LAUNCH_OUTPUT" | jq

$HTTP "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"

sleep 5

$HTTP --stream "$INSTANCE_HOST/instances/$INSTANCE_ID/logs" "Authorization: Bearer $ACCESS_TOKEN"
