#!/usr/bin/env bash

set -euo pipefail

STACK_ID=$1

$HTTP "$INSTANCE_HOST/stacks/$STACK_ID" "Authorization: Bearer $ACCESS_TOKEN"
