#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=minio

echo "{
  \"stackName\": \"$STACK_NAME\"
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
