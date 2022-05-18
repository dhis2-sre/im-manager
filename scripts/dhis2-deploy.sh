#!/usr/bin/env bash

set -euo pipefail

READINESS_PROBE_INITIAL_DELAY_SECONDS=100
LIVENESS_PROBE_INITIAL_DELAY_SECONDS=100
IMAGE_REPOSITORY=core
IMAGE_TAG=2.36.0-tomcat-8.5.34-jre8-alpine
DATABASE_SIZE=30Gi
PGADMIN_INSTALL=false
DATABASE_ID=3

INSTANCE_NAME=$1
GROUP_NAME=$2

GROUP_ID=$($HTTP --check-status "$INSTANCE_HOST/groups-name-to-id/$GROUP_NAME" "Authorization: Bearer $ACCESS_TOKEN")
INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_ID/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$INSTANCE_NAME\",
  \"groupId\": $GROUP_ID,
  \"stackId\": 1,
  \"optionalParameters\": [
    {
      \"stackParameterId\": \"READINESS_PROBE_INITIAL_DELAY_SECONDS\",
      \"value\": \"$READINESS_PROBE_INITIAL_DELAY_SECONDS\"
    },
    {
      \"stackParameterId\": \"LIVENESS_PROBE_INITIAL_DELAY_SECONDS\",
      \"value\": \"$LIVENESS_PROBE_INITIAL_DELAY_SECONDS\"
    },
    {
      \"stackParameterId\": \"IMAGE_REPOSITORY\",
      \"value\": \"$IMAGE_REPOSITORY\"
    },
    {
      \"stackParameterId\": \"IMAGE_TAG\",
      \"value\": \"$IMAGE_TAG\"
    },
    {
      \"stackParameterId\": \"DATABASE_SIZE\",
      \"value\": \"$DATABASE_SIZE\"
    },
    {
      \"stackParameterId\": \"PGADMIN_INSTALL\",
      \"value\": \"$PGADMIN_INSTALL\"
    }
  ],
  \"requiredParameters\": [
    {
      \"stackParameterId\": \"DATABASE_ID\",
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
