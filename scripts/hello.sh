#!/usr/bin/env bash

set -euo pipefail

GROUP_ID=2
STACK_ID=5
REQUIRED_PARAMETER_ID=4
REQUIRED_PARAMETER_VALUE="0.5.0"

function int_handler {
  $HTTP delete "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
  trap - EXIT
  exit
}

trap int_handler INT
trap int_handler EXIT

# Create instance
INSTANCE_OUTPUT=$(echo "{
  \"name\": \"$1\",
  \"groupId\": $GROUP_ID,
  \"stackId\": $STACK_ID
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN")

INSTANCE_ID=$(echo "$INSTANCE_OUTPUT" | jq -r '.ID')

# Show created instance
$HTTP "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"

# Deploy instance
echo "{
  \"requiredParameters\": [
    {
      \"stackParameterId\": $REQUIRED_PARAMETER_ID,
      \"value\": \"$REQUIRED_PARAMETER_VALUE\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"

sleep 5

# Stream instance logs
$HTTP --stream "$INSTANCE_HOST/instances/$INSTANCE_ID/logs" "Authorization: Bearer $ACCESS_TOKEN"
