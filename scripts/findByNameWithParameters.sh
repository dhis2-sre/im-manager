#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
NAME=$2

INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

$HTTP "$INSTANCE_HOST/instances/$INSTANCE_ID/parameters" "Authorization: Bearer $ACCESS_TOKEN"
