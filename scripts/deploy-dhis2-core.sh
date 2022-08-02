#!/usr/bin/env bash

set -euo pipefail

STACK=dhis2-core

GROUP=$1
SOURCE_INSTANCE=$2
DESTINATION_INSTANCE=$3

# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=26
STARTUP_PROBE_PERIOD_SECONDS=5
IMAGE_TAG=2.36.0-tomcat-8.5.34-jre8-alpine

SOURCE_INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP/$SOURCE_INSTANCE" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$DESTINATION_INSTANCE\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"sourceInstance\": $SOURCE_INSTANCE_ID,
  \"optionalParameters\": [
    {
      \"name\": \"STARTUP_PROBE_FAILURE_THRESHOLD\",
      \"value\": \"$STARTUP_PROBE_FAILURE_THRESHOLD\"
    },
    {
      \"name\": \"STARTUP_PROBE_PERIOD_SECONDS\",
      \"value\": \"$STARTUP_PROBE_PERIOD_SECONDS\"
    },
    {
      \"name\": \"IMAGE_TAG\",
      \"value\": \"$IMAGE_TAG\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
