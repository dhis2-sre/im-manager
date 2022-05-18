#!/usr/bin/env bash

set -euo pipefail

STACK_ID=5 # Id of the whoami-go stack. Run ./stacks.sh for a list of all stacks
CHART_VERSION=0.5.0
GROUP_NAME=whoami

GROUP_ID=$($HTTP --check-status "$INSTANCE_HOST/groups-name-to-id/$GROUP_NAME" "Authorization: Bearer $ACCESS_TOKEN")

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
      \"stackParameter\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"

sleep 5

# Stream instance logs
$HTTP --stream "$INSTANCE_HOST/instances/$INSTANCE_ID/logs" "Authorization: Bearer $ACCESS_TOKEN"
