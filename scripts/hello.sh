#!/usr/bin/env bash

set -euo pipefail

STACK_NAME=whoami-go
CHART_VERSION=0.5.0
GROUP_NAME=whoami

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
  \"groupName\": \"$GROUP_NAME\",
  \"stackName\": \"$STACK_NAME\"
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN")

INSTANCE_ID=$(echo "$INSTANCE_OUTPUT" | jq -r '.ID')

# Show created instance
$HTTP "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"

# Deploy instance
echo "{
  \"requiredParameters\": [
    {
      \"name\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"

sleep 5

# Stream instance logs
$HTTP --stream "$INSTANCE_HOST/instances/$INSTANCE_ID/logs" "Authorization: Bearer $ACCESS_TOKEN"
