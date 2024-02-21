#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2-core

GROUP=$1
NAME=$2
SOURCE_INSTANCE=${3:-""}

MIN_READY_SECONDS=${MIN_READY_SECONDS:-120}
# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=${STARTUP_PROBE_FAILURE_THRESHOLD:-26}
STARTUP_PROBE_PERIOD_SECONDS=${STARTUP_PROBE_PERIOD_SECONDS:-5}
LIVENESS_PROBE_TIMEOUT_SECONDS=${LIVENESS_PROBE_TIMEOUT_SECONDS:-1}
READINESS_PROBE_TIMEOUT_SECONDS=${LIVENESS_PROBE_TIMEOUT_SECONDS:-1}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-core}
IMAGE_TAG=${IMAGE_TAG:-2.40.2}
CHART_VERSION=${CHART_VERSION:-0.12.1}
INSTANCE_TTL=${INSTANCE_TTL:-0}
PUBLIC=${PUBLIC:-false}
FLYWAY_MIGRATE_OUT_OF_ORDER=${FLYWAY_MIGRATE_OUT_OF_ORDER:-false}
FLYWAY_REPAIR_BEFORE_MIGRATION=${FLYWAY_REPAIR_BEFORE_MIGRATION:-false}
RESOURCES_REQUESTS_CPU=${RESOURCES_REQUESTS_CPU:-250m}
RESOURCES_REQUESTS_MEMORY=${RESOURCES_REQUESTS_MEMORY:-1500Mi}

SOURCE_INSTANCE_ID=0
if [ -n "$SOURCE_INSTANCE" ]; then
  SOURCE_INSTANCE_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$SOURCE_INSTANCE" "Authorization: Bearer $ACCESS_TOKEN")
fi

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"ttl\": $INSTANCE_TTL,
  \"public\": $PUBLIC,
  \"sourceInstance\": $SOURCE_INSTANCE_ID,
  \"parameters\": [
    {
      \"name\": \"MIN_READY_SECONDS\",
      \"value\": \"$MIN_READY_SECONDS\"
    },
    {
      \"name\": \"STARTUP_PROBE_FAILURE_THRESHOLD\",
      \"value\": \"$STARTUP_PROBE_FAILURE_THRESHOLD\"
    },
    {
      \"name\": \"STARTUP_PROBE_PERIOD_SECONDS\",
      \"value\": \"$STARTUP_PROBE_PERIOD_SECONDS\"
    },
    {
      \"name\": \"LIVENESS_PROBE_TIMEOUT_SECONDS\",
      \"value\": \"$LIVENESS_PROBE_TIMEOUT_SECONDS\"
    },
    {
      \"name\": \"READINESS_PROBE_TIMEOUT_SECONDS\",
      \"value\": \"$READINESS_PROBE_TIMEOUT_SECONDS\"
    },
    {
      \"name\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
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
      \"name\": \"FLYWAY_MIGRATE_OUT_OF_ORDER\",
      \"value\": \"$FLYWAY_MIGRATE_OUT_OF_ORDER\"
    },
    {
      \"name\": \"FLYWAY_REPAIR_BEFORE_MIGRATION\",
      \"value\": \"$FLYWAY_REPAIR_BEFORE_MIGRATION\"
    },
    {
      \"name\": \"RESOURCES_REQUESTS_CPU\",
      \"value\": \"$RESOURCES_REQUESTS_CPU\"
    },
    {
      \"name\": \"RESOURCES_REQUESTS_MEMORY\",
      \"value\": \"$RESOURCES_REQUESTS_MEMORY\"
    }
  ]
}" | $HTTP post "$IM_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
