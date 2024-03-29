#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
shift
shift
DESCRIPTION=${*:-""}
INSTANCE_TTL=${INSTANCE_TTL:-0}

echo "{
  \"name\": \"$NAME\",
  \"group\": \"$GROUP\",
  \"description\": \"$DESCRIPTION\",
  \"ttl\": $INSTANCE_TTL
}" | $HTTP post "$IM_HOST/deployments" "Authorization: Bearer $ACCESS_TOKEN"
