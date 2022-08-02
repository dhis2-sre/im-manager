#!/usr/bin/env bash

set -euo pipefail

STACK=dhis2

GROUP=$1
NAME=$2

# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
STARTUP_PROBE_FAILURE_THRESHOLD=26
STARTUP_PROBE_PERIOD_SECONDS=5
IMAGE_REPOSITORY=core
IMAGE_TAG=2.36.0-tomcat-8.5.34-jre8-alpine
DATABASE_SIZE=30Gi
PGADMIN_INSTALL=false
DATABASE_ID=1

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
      \"name\": \"DATABASE_SIZE\",
      \"value\": \"$DATABASE_SIZE\"
    },
    {
      \"name\": \"PGADMIN_INSTALL\",
      \"value\": \"$PGADMIN_INSTALL\"
    }
  ],
  \"requiredParameters\": [
    {
      \"name\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
