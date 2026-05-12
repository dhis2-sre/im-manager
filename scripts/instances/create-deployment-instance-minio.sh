#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
STACK_NAME=minio

MINIO_STORAGE_SIZE=${MINIO_STORAGE_SIZE:-8Gi}
MINIO_CHART_VERSION=${MINIO_CHART_VERSION:-14.7.5}
IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}

echo "{
  \"stackName\": \"$STACK_NAME\",
  \"parameters\": {
    \"MINIO_STORAGE_SIZE\": {
      \"value\": \"$MINIO_STORAGE_SIZE\"
    },
    \"MINIO_CHART_VERSION\": {
      \"value\": \"$MINIO_CHART_VERSION\"
    },
    \"IMAGE_PULL_POLICY\": {
      \"value\": \"$IMAGE_PULL_POLICY\"
    }
}" | $HTTP post "$IM_HOST/deployments/$DEPLOYMENT_ID/instance" "Authorization: Bearer $ACCESS_TOKEN"
