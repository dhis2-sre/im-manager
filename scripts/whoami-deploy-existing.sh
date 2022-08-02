#!/usr/bin/env bash

set -euo pipefail

CHART_VERSION="0.5.0"

GROUP=$1
NAME=$2
TTL=${3:-10}

INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

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
       \"value\": \"$TTL\"
    }
  ]
}" | $HTTP put "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
