#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
TTL=$3
DESCRIPTION=${4:-}

DEPLOYMENT_ID=$(./findByName.sh "$GROUP" "$NAME" | jq -r '.id')

echo "{
  \"ttl\": $TTL,
  \"description\": \"$DESCRIPTION\"
}" | $HTTP put "$IM_HOST/deployments/$DEPLOYMENT_ID" "Authorization: Bearer $ACCESS_TOKEN"