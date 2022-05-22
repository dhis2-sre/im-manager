#!/usr/bin/env bash

set -euo pipefail

INSTANCE_NAME=$1
GROUP_NAME=$2
SELECTOR=${3:-""}

INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

$HTTP --stream "$INSTANCE_HOST/instances/$INSTANCE_ID/logs?selector=$SELECTOR" "Authorization: Bearer $ACCESS_TOKEN"
