#!/usr/bin/env bash

set -euo pipefail

IMAGE_TAG=2.36.0-tomcat-8.5.34-jre8-alpine
READINESS_PROBE_INITIAL_DELAY_SECONDS=100
LIVENESS_PROBE_INITIAL_DELAY_SECONDS=100

FIRST_INSTANCE_NAME=$1
SECOND_INSTANCE_NAME=$2
GROUP_NAME=$3
STACK_NAME=dhis2-core

FIRST_INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$FIRST_INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")
SECOND_INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$SECOND_INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$SECOND_INSTANCE_NAME\",
  \"groupName\": \"$GROUP_NAME\",
  \"stackName\": \"$STACK_NAME\",
  \"optionalParameters\": [
    {
      \"name\": \"READINESS_PROBE_INITIAL_DELAY_SECONDS\",
      \"value\": \"$READINESS_PROBE_INITIAL_DELAY_SECONDS\"
    },
    {
      \"name\": \"LIVENESS_PROBE_INITIAL_DELAY_SECONDS\",
      \"value\": \"$LIVENESS_PROBE_INITIAL_DELAY_SECONDS\"
    },
    {
      \"name\": \"IMAGE_TAG\",
      \"value\": \"$IMAGE_TAG\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$FIRST_INSTANCE_ID/link/$SECOND_INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
