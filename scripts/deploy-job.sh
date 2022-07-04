#!/usr/bin/env bash

set -euo pipefail

INSTANCE_NAME=$1
GROUP_NAME=$2

INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

JOB=database/seed

PAYLOAD='{\"databaseId\": \"11\"}'

echo "{
  \"requiredParameters\": [
    {
      \"name\": \"COMMAND\",
      \"value\": \"$JOB\"
    }
  ],
  \"optionalParameters\": [
    {
      \"name\": 10,
      \"value\": \"300\"
    },
    {
      \"name\": 21,
      \"value\": \"$PAYLOAD\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
