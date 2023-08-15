#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
shift
shift
DESCRIPTION=${*:-""}

# container(s) in dhis2 pod will be restarted after that due to restartPolicy
# 5*26=130s
CHAIN_TTL=${CHAIN_TTL:-0}
PUBLIC=${PUBLIC:-false}

echo "{
  \"name\": \"$NAME\",
  \"group\": \"$GROUP\",
  \"description\": \"$DESCRIPTION\",
  \"ttl\": $CHAIN_TTL
}" | $HTTP post "$IM_HOST/chains" "Authorization: Bearer $ACCESS_TOKEN"
