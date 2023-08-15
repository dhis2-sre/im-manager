#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

CHAIN_ID=$1
STACK_NAME=$2
PRESET=${3:-false}

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"preset\": $PRESET,
  \"parameters\": {
    \"REPLICA_COUNT\": {
      \"value\": \"1\"
    },
    \"IMAGE_PULL_POLICY\": {
      \"value\": \"Always\"
    }
  }
}" | $HTTP post "$IM_HOST/chains/$CHAIN_ID/link" "Authorization: Bearer $ACCESS_TOKEN"
