#!/usr/bin/env bash

set -euo pipefail

default_tag="2.37.4-tomcat-8.5.34-jre8-alpine"
tag=${DHIS2_IMAGE_TAG:-$default_tag}

INSTANCE_NAME=$1
GROUP_NAME=$2
DATABASE_ID=$3

GROUP_ID=$($HTTP get "$INSTANCE_HOST/groups-name-to-id/$GROUP_NAME" "Authorization: Bearer $ACCESS_TOKEN")
INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP_ID/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$INSTANCE_NAME\",
  \"groupId\": $GROUP_ID,
  \"stackId\": 1,
  \"requiredParameters\": [
    {
      \"stackParameterId\": 1,
      \"value\": \"$DATABASE_ID\"
    }
  ],
  \"optionalParameters\": [
      {
        \"stackParameterId\": 4,
        \"value\": \"$tag\"
      }
    ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
