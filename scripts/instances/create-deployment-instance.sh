#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=$2

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"parameters\": {
    \"REPLICA_COUNT\": {
      \"value\": \"1\"
    },
    \"IMAGE_PULL_POLICY\": {
      \"value\": \"Always\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
