#!/usr/bin/env bash

set -e

HTTP="http --verify=no --check-status"

IMAGE_TAG="2.36.0-tomcat-8.5.34-jre8-alpine"
SEED_PATH="2.36.0/dhis2-db-sierra-leone.sql.gz"

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
      \"stackParameterId\": 3,
      \"value\": \"$SEED_PATH\"
    },
    {
      \"stackParameterId\": 1,
      \"value\": \"$IMAGE_TAG\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
