#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=dhis2

GROUP=$1
NAME=$2
shift
shift
DESCRIPTION=${*:-""}

# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=${STARTUP_PROBE_FAILURE_THRESHOLD:-26}
STARTUP_PROBE_PERIOD_SECONDS=${STARTUP_PROBE_PERIOD_SECONDS:-5}
LIVENESS_PROBE_TIMEOUT_SECONDS=${LIVENESS_PROBE_TIMEOUT_SECONDS:-1}
READINESS_PROBE_TIMEOUT_SECONDS=${LIVENESS_PROBE_TIMEOUT_SECONDS:-1}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-core}
IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}
IMAGE_TAG=${IMAGE_TAG:-2.40.0}
DATABASE_ID=${DATABASE_ID:-whoami-sierra-leone-2-40-0-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-30Gi}
INSTALL_REDIS=${INSTALL_REDIS:-false}
INSTANCE_TTL=${INSTANCE_TTL:-0}
FLYWAY_MIGRATE_OUT_OF_ORDER=${FLYWAY_MIGRATE_OUT_OF_ORDER:-false}
FLYWAY_REPAIR_BEFORE_MIGRATION=${FLYWAY_REPAIR_BEFORE_MIGRATION:-false}
CORE_RESOURCES_REQUESTS_CPU=${CORE_RESOURCES_REQUESTS_CPU:-250m}
CORE_RESOURCES_REQUESTS_MEMORY=${CORE_RESOURCES_REQUESTS_MEMORY:-256Mi}
DB_RESOURCES_REQUESTS_CPU=${DB_RESOURCES_REQUESTS_CPU:-250m}
DB_RESOURCES_REQUESTS_MEMORY=${DB_RESOURCES_REQUESTS_MEMORY:-256Mi}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"description\": \"$DESCRIPTION\",
  \"stackName\": \"$STACK\",
  \"ttl\": $INSTANCE_TTL,
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
      \"name\": \"LIVENESS_PROBE_TIMEOUT_SECONDS\",
      \"value\": \"$LIVENESS_PROBE_TIMEOUT_SECONDS\"
    },
    {
      \"name\": \"READINESS_PROBE_TIMEOUT_SECONDS\",
      \"value\": \"$READINESS_PROBE_TIMEOUT_SECONDS\"
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
      \"name\": \"CORE_RESOURCES_REQUESTS_CPU\",
      \"value\": \"$CORE_RESOURCES_REQUESTS_CPU\"
    },
    {
      \"name\": \"CORE_RESOURCES_REQUESTS_MEMORY\",
      \"value\": \"$CORE_RESOURCES_REQUESTS_MEMORY\"
    },
    {
      \"name\": \"DB_RESOURCES_REQUESTS_CPU\",
      \"value\": \"$DB_RESOURCES_REQUESTS_CPU\"
    },
    {
      \"name\": \"DB_RESOURCES_REQUESTS_MEMORY\",
      \"value\": \"$DB_RESOURCES_REQUESTS_MEMORY\"
    }
  ],
  \"requiredParameters\": [
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP post "$IM_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
