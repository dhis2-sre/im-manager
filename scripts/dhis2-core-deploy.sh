#!/usr/bin/env bash

set -euo pipefail

IMAGE_TAG=2.36.0-tomcat-8.5.34-jre8-alpine
READINESS_PROBE_INITIAL_DELAY_SECONDS=100
LIVENESS_PROBE_INITIAL_DELAY_SECONDS=100

INSTANCE_NAME=$1
GROUP_NAME=$2
STACK_NAME=dhis2-core

INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$INSTANCE_NAME\",
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
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
