#!/usr/bin/env bash

set -euo pipefail

IMAGE_TAG_PARAMETER_ID=23
IMAGE_TAG="2.36.0-tomcat-8.5.34-jre8-alpine"

DATABASE_HOSTNAME_PARAMETER_ID=3
DATABASE_HOSTNAME="sl-db-1-database-postgresql.whoami.svc"

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
      \"stackParameterId\": $IMAGE_TAG_PARAMETER_ID,
      \"value\": \"$IMAGE_TAG\"
    }
  ],
  \"requiredParameters\": [
    {
      \"stackParameterId\": $DATABASE_HOSTNAME_PARAMETER_ID,
      \"value\": \"$DATABASE_HOSTNAME\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
