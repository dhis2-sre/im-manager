#!/usr/bin/env bash

set -euo pipefail

INSTANCE_NAME=$1
GROUP_NAME=$2

GROUP_ID=$($HTTP --check-status "$INSTANCE_HOST/groups-name-to-id/$GROUP_NAME" "Authorization: Bearer $ACCESS_TOKEN")
INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_ID/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

JOB=database/seed

PAYLOAD='{\"databaseId\": \"11\"}'

echo "{
  \"requiredParameters\": [
    {
      \"stackParameterId\": 5,
      \"value\": \"$JOB\"
    }
  ],
  \"optionalParameters\": [
    {
      \"stackParameterId\": 10,
      \"value\": \"300\"
    },
    {
      \"stackParameterId\": 21,
      \"value\": \"$PAYLOAD\"
    }
  ]
}"

echo "{
  \"requiredParameters\": [
    {
      \"stackParameterId\": 5,
      \"value\": \"$JOB\"
    }
  ],
  \"optionalParameters\": [
    {
      \"stackParameterId\": 10,
      \"value\": \"300\"
    },
    {
      \"stackParameterId\": 21,
      \"value\": \"$PAYLOAD\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
