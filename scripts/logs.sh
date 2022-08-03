#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
NAME=$2
SELECTOR=${3:-""}

INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

$HTTP --stream "$INSTANCE_HOST/instances/$INSTANCE_ID/logs?selector=$SELECTOR" "Authorization: Bearer $ACCESS_TOKEN"
