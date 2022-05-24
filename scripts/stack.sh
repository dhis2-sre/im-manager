#!/usr/bin/env bash

set -euo pipefail

STACK_NAME=$1

$HTTP "$INSTANCE_HOST/stacks/$STACK_NAME" "Authorization: Bearer $ACCESS_TOKEN"
