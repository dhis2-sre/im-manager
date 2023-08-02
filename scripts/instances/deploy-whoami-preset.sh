#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=whoami-go

GROUP=$1
NAME=$2
CHART_VERSION=${CHART_VERSION:-0.8.0}
INSTANCE_TTL=${INSTANCE_TTL:-0}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"ttl\": $INSTANCE_TTL,
  \"parameters\": [
    {
      \"name\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    }
  ]
}" | $HTTP post "$IM_HOST/instances?preset=true" "Authorization: Bearer $ACCESS_TOKEN"
