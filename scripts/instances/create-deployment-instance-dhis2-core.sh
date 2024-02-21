#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=dhis2-core

MIN_READY_SECONDS=${MIN_READY_SECONDS:-120}

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"parameters\": {
    \"MIN_READY_SECONDS\": {
      \"value\": \"$MIN_READY_SECONDS\"
    },
    \"IMAGE_PULL_POLICY\": {
      \"value\": \"Always\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
