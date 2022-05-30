#!/usr/bin/env bash

set -euo pipefail

INSTANCE_NAME=$1
GROUP_NAME=$2

INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

$HTTP "$INSTANCE_HOST/instances/$INSTANCE_ID/parameters" "Authorization: Bearer $ACCESS_TOKEN"
