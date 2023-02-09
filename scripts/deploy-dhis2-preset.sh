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
IMAGE_TAG=${IMAGE_TAG:-2.38.1.1-tomcat-9.0-jdk11-openjdk-slim}
IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}
DATABASE_SIZE=${DATABASE_SIZE:-10Gi}
INSTALL_PGADMIN=${INSTALL_PGADMIN:-false}
DATABASE_ID=${DATABASE_ID:-2}
INSTANCE_TTL=${INSTANCE_TTL:-""}

echo "{
  \"name\": \"$NAME\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
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
      \"name\": \"IMAGE_PULL_POLICY\",
      \"value\": \"$IMAGE_PULL_POLICY\"
    },
    {
      \"name\": \"DATABASE_SIZE\",
      \"value\": \"$DATABASE_SIZE\"
    },
    {
      \"name\": \"INSTALL_PGADMIN\",
      \"value\": \"$INSTALL_PGADMIN\"
    },
    {
       \"name\": \"INSTANCE_TTL\",
       \"value\": \"$INSTANCE_TTL\"
    }
  ],
  \"requiredParameters\": [
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP post "$IM_HOST/instances?preset=true" "Authorization: Bearer $ACCESS_TOKEN"
