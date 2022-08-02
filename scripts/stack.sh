#!/usr/bin/env bash

set -euo pipefail

STACK=$1

$HTTP "$INSTANCE_HOST/stacks/$STACK" "Authorization: Bearer $ACCESS_TOKEN"
