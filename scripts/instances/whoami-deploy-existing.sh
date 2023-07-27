#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

CHART_VERSION="0.5.0"

GROUP=$1
NAME=$2
TTL=${3:-10}

INSTANCE_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"parameters\": [
    {
       \"name\": \"INSTANCE_TTL\",
       \"value\": \"$TTL\"
    },
    {
      \"name\": \"CHART_VERSION\",
      \"value\": \"$CHART_VERSION\"
    }
  ]
}" | $HTTP put "$IM_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
