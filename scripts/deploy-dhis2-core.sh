#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2-core

GROUP=$1
SOURCE_INSTANCE=$2
DESTINATION_INSTANCE=$3

# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=${STARTUP_PROBE_FAILURE_THRESHOLD:-26}
STARTUP_PROBE_PERIOD_SECONDS=${STARTUP_PROBE_PERIOD_SECONDS:-5}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-core}
IMAGE_TAG=${IMAGE_TAG:-2.38.1.1-tomcat-9.0-jdk11-openjdk-slim}
INSTANCE_TTL=${INSTANCE_TTL:-""}

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
      \"name\": \"IMAGE_REPOSITORY\",
      \"value\": \"$IMAGE_REPOSITORY\"
    },
    {
      \"name\": \"IMAGE_TAG\",
      \"value\": \"$IMAGE_TAG\"
    },
    {
      \"name\": \"INSTANCE_TTL\",
      \"value\": \"$INSTANCE_TTL\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
