#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=dhis2-core

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"parameters\": {
    \"IMAGE_PULL_POLICY\": {
      \"value\": \"Always\"
    },
    \"IMAGE_REPOSITORY\": {
      \"value\": \"core-dev\"
    },
    \"IMAGE_TAG\": {
      \"value\": \"latest\"
    }
  }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
