#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=whoami-go

GROUP=$1
NAME=$2
TTL=${3:-300}
CHART_VERSION=${CHART_VERSION:-0.9.0}
IMAGE_TAG=${IMAGE_TAG:-0.6.0}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"ttl\": $TTL,
  \"parameters\": [
    {
      \"name\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    },
    {
      \"name\": \"IMAGE_TAG\",
      \"value\": \"$IMAGE_TAG\"
    }
  ]
}" | $HTTP post "$IM_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
