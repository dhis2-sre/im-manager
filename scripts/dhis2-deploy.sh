#!/usr/bin/env bash

set -euo pipefail

READINESS_PROBE_INITIAL_DELAY_SECONDS=0
LIVENESS_PROBE_INITIAL_DELAY_SECONDS=0
IMAGE_REPOSITORY=core
IMAGE_TAG=2.36.0-tomcat-8.5.34-jre8-alpine
DATABASE_SIZE=30Gi
PGADMIN_INSTALL=false
DATABASE_ID=1

INSTANCE_NAME=$1
GROUP_NAME=$2
STACK_NAME=dhis2

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
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
