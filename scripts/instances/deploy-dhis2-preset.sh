#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2

GROUP=$1
NAME=$2

# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=${STARTUP_PROBE_FAILURE_THRESHOLD:-26}
STARTUP_PROBE_PERIOD_SECONDS=${STARTUP_PROBE_PERIOD_SECONDS:-5}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-core}
IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}
IMAGE_TAG=${IMAGE_TAG:-2.40.0}
DATABASE_ID=${DATABASE_ID:-whoami-sierra-leone-2-40-0-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-10Gi}
INSTANCE_TTL=${INSTANCE_TTL:-0}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"ttl\": $INSTANCE_TTL,
  \"parameters\": [
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
      \"name\": \"IMAGE_PULL_POLICY\",
      \"value\": \"$IMAGE_PULL_POLICY\"
    },
    {
      \"name\": \"DATABASE_SIZE\",
      \"value\": \"$DATABASE_SIZE\"
    },
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP post "$IM_HOST/instances?preset=true" "Authorization: Bearer $ACCESS_TOKEN"
