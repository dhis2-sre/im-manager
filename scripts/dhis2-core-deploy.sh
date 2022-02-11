#!/usr/bin/env bash

set -euo pipefail

IMAGE_TAG="2.36.0-tomcat-8.5.34-jre8-alpine"

INSTANCE_NAME=$1
GROUP_NAME=$2

GROUP_ID=$($HTTP --check-status "$INSTANCE_HOST/groups-name-to-id/$GROUP_NAME" "Authorization: Bearer $ACCESS_TOKEN")
INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_ID/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$INSTANCE_NAME\",
  \"groupId\": $GROUP_ID,
  \"stackId\": 1,
  \"optionalParameters\": [
    {
      \"stackParameterId\": 1,
      \"value\": \"$IMAGE_TAG\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
