#!/usr/bin/env bash

set -euo pipefail

DATABASE_SIZE="30Gi"
DATABASE_SIZE_PARAMETER_ID=10

READINESS_PROBE_INITIAL_DELAY_SECONDS=100
READINESS_PROBE_INITIAL_DELAY_SECONDS_PARAMETER_ID=12

LIVENESS_PROBE_INITIAL_DELAY_SECONDS=100
LIVENESS_PROBE_INITIAL_DELAY_SECONDS_PARAMETER_ID=5

IMAGE_REPOSITORY=core
IMAGE_REPOSITORY_PARAMETER_ID=11

IMAGE_TAG="2.36.0-tomcat-8.5.34-jre8-alpine"
IMAGE_TAG_PARAMETER_ID=1

DATABASE_ID=1
DATABASE_ID_PARAMETER_ID=1

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
      \"stackParameterId\": $READINESS_PROBE_INITIAL_DELAY_SECONDS_PARAMETER_ID,
      \"value\": \"$READINESS_PROBE_INITIAL_DELAY_SECONDS\"
    },
    {
      \"stackParameterId\": $LIVENESS_PROBE_INITIAL_DELAY_SECONDS_PARAMETER_ID,
      \"value\": \"$LIVENESS_PROBE_INITIAL_DELAY_SECONDS\"
    },
    {
      \"stackParameterId\": $IMAGE_REPOSITORY_PARAMETER_ID,
      \"value\": \"$IMAGE_REPOSITORY\"
    },
    {
      \"stackParameterId\": $IMAGE_TAG_PARAMETER_ID,
      \"value\": \"$IMAGE_TAG\"
    },
    {
      \"stackParameterId\": $DATABASE_SIZE_PARAMETER_ID,
      \"value\": \"$DATABASE_SIZE\"
    }
  ],
  \"requiredParameters\": [
    {
      \"stackParameterId\": $DATABASE_ID_PARAMETER_ID,
      \"value\": \"$DATABASE_ID\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
