#!/usr/bin/env bash

set -euo pipefail

STACK=whoami-go

GROUP=$1
NAME=$2
CHART_VERSION=${3:-"0.5.0"}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"requiredParameters\": [
    {
      \"name\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
