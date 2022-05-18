#!/usr/bin/env bash

set -euo pipefail

CHART_VERSION="0.5.0"

INSTANCE_NAME=$1
GROUP_NAME=$2

GROUP_ID=$($HTTP --check-status "$INSTANCE_HOST/groups-name-to-id/$GROUP_NAME" "Authorization: Bearer $ACCESS_TOKEN")
INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_ID/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"requiredParameters\": [
    {
      \"stackParameter\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$INSTANCE_ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
