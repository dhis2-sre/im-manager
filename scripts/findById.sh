#!/usr/bin/env bash

set -euo pipefail

INSTANCE_ID=$1

$HTTP "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
