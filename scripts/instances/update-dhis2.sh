#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2

# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=${STARTUP_PROBE_FAILURE_THRESHOLD:-26}
STARTUP_PROBE_PERIOD_SECONDS=${STARTUP_PROBE_PERIOD_SECONDS:-5}
LIVENESS_PROBE_TIMEOUT_SECONDS=${LIVENESS_PROBE_TIMEOUT_SECONDS:-1}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-core}
IMAGE_TAG=${IMAGE_TAG:-2.40.0}
IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}
DATABASE_SIZE=${DATABASE_SIZE:-30Gi}
INSTALL_REDIS=${INSTALL_REDIS:-false}
DATABASE_ID=${DATABASE_ID:-whoami-sierra-leone-2-40-0-sql-gz}
INSTANCE_TTL=${INSTANCE_TTL:-0}
FLYWAY_MIGRATE_OUT_OF_ORDER=${FLYWAY_MIGRATE_OUT_OF_ORDER:-false}
FLYWAY_REPAIR_BEFORE_MIGRATION=${FLYWAY_REPAIR_BEFORE_MIGRATION:-false}

INSTANCE_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
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
      \"name\": \"LIVENESS_PROBE_TIMEOUT_SECONDS\",
      \"value\": \"$LIVENESS_PROBE_TIMEOUT_SECONDS\"
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
      \"name\": \"INSTALL_REDIS\",
      \"value\": \"$INSTALL_REDIS\"
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
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP put "$IM_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
