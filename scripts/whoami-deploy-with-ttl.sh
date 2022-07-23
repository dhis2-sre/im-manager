#!/usr/bin/env bash

set -euo pipefail

CHART_VERSION="0.5.0"

INSTANCE_NAME=$1
GROUP_NAME=$2
INSTANCE_TTL=${3-10}

INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"requiredParameters\": [
    {
      \"name\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    }
  ],
  \"optionalParameters\": [
    {
       \"name\": \"INSTANCE_TTL\",
       \"value\": \"$INSTANCE_TTL\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
