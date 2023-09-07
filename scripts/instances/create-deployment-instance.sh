#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
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
}" | $HTTP post "$IM_HOST/chains/$DEPLOYMENT_ID/link" "Authorization: Bearer $ACCESS_TOKEN"