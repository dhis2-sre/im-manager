#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
shift
shift
DESCRIPTION=${*:-""}

INSTANCE_TTL=${INSTANCE_TTL:-0}
PUBLIC=${PUBLIC:-false}

DATABASE_ID=${DATABASE_ID:-test-dbs-sierra-leone-2-40-2-sql-gz}
DATABASE_SIZE=${DATABASE_SIZE:-20Gi}
DB_RESOURCES_REQUESTS_CPU=${DB_RESOURCES_REQUESTS_CPU:-250m}
DB_RESOURCES_REQUESTS_MEMORY=${DB_RESOURCES_REQUESTS_MEMORY:-256Mi}

MIN_READY_SECONDS=${MIN_READY_SECONDS:-120}
# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=${STARTUP_PROBE_FAILURE_THRESHOLD:-26}
STARTUP_PROBE_PERIOD_SECONDS=${STARTUP_PROBE_PERIOD_SECONDS:-5}
LIVENESS_PROBE_TIMEOUT_SECONDS=${LIVENESS_PROBE_TIMEOUT_SECONDS:-1}
READINESS_PROBE_TIMEOUT_SECONDS=${LIVENESS_PROBE_TIMEOUT_SECONDS:-1}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-core}
IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}
IMAGE_TAG=${IMAGE_TAG:-2.40.2}
CORE_RESOURCES_REQUESTS_CPU=${CORE_RESOURCES_REQUESTS_CPU:-250m}
CORE_RESOURCES_REQUESTS_MEMORY=${CORE_RESOURCES_REQUESTS_MEMORY:-1500Mi}
FLYWAY_MIGRATE_OUT_OF_ORDER=${FLYWAY_MIGRATE_OUT_OF_ORDER:-false}
FLYWAY_REPAIR_BEFORE_MIGRATION=${FLYWAY_REPAIR_BEFORE_MIGRATION:-false}
ENABLE_QUERY_LOGGING=${ENABLE_QUERY_LOGGING:-false}
ALLOW_SUSPEND=${ALLOW_SUSPEND:-true}

DEPLOYMENT_ID=$(echo "{
  \"name\": \"$NAME\",
  \"group\": \"$GROUP\",
  \"description\": \"$DESCRIPTION\",
  \"ttl\": $INSTANCE_TTL
}" | $HTTP post "$IM_HOST/deployments" "Authorization: Bearer $ACCESS_TOKEN" | jq -r '.id')

echo "{
  \"stackName\": \"dhis2-db\",
  \"parameters\": {
    \"DATABASE_ID\": {
      \"value\": \"$DATABASE_ID\"
    },
    \"DATABASE_SIZE\": {
      \"value\": \"$DATABASE_SIZE\"
    },
    \"RESOURCES_REQUESTS_CPU\": {
      \"value\": \"$DB_RESOURCES_REQUESTS_CPU\"
    },
    \"RESOURCES_REQUESTS_MEMORY\": {
      \"value\": \"$DB_RESOURCES_REQUESTS_MEMORY\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"

echo "{
  \"stackName\": \"dhis2-core\",
  \"public\": $PUBLIC,
  \"parameters\": {
    \"MIN_READY_SECONDS\": {
      \"value\": \"$MIN_READY_SECONDS\"
    },
    \"IMAGE_PULL_POLICY\": {
      \"value\": \"$IMAGE_PULL_POLICY\"
    },
    \"STARTUP_PROBE_FAILURE_THRESHOLD\": {
      \"value\": \"$STARTUP_PROBE_FAILURE_THRESHOLD\"
    },
    \"STARTUP_PROBE_PERIOD_SECONDS\": {
      \"value\": \"$STARTUP_PROBE_PERIOD_SECONDS\"
    },
    \"LIVENESS_PROBE_TIMEOUT_SECONDS\": {
      \"value\": \"$LIVENESS_PROBE_TIMEOUT_SECONDS\"
    },
    \"READINESS_PROBE_TIMEOUT_SECONDS\": {
      \"value\": \"$READINESS_PROBE_TIMEOUT_SECONDS\"
    },
    \"IMAGE_REPOSITORY\": {
      \"value\": \"$IMAGE_REPOSITORY\"
    },
    \"IMAGE_PULL_POLICY\": {
      \"value\": \"$IMAGE_PULL_POLICY\"
    },
    \"IMAGE_TAG\": {
      \"value\": \"$IMAGE_TAG\"
    },
    \"RESOURCES_REQUESTS_CPU\": {
      \"value\": \"$CORE_RESOURCES_REQUESTS_CPU\"
    },
    \"RESOURCES_REQUESTS_MEMORY\": {
      \"value\": \"$CORE_RESOURCES_REQUESTS_MEMORY\"
    },
    \"FLYWAY_MIGRATE_OUT_OF_ORDER\": {
      \"value\": \"$FLYWAY_MIGRATE_OUT_OF_ORDER\"
    },
    \"FLYWAY_REPAIR_BEFORE_MIGRATION\": {
      \"value\": \"$FLYWAY_REPAIR_BEFORE_MIGRATION\"
    },
    \"ENABLE_QUERY_LOGGING\": {
      \"value\": \"$ENABLE_QUERY_LOGGING\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"

$HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
