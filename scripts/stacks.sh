#!/usr/bin/env bash

set -euo pipefail

$HTTP "$INSTANCE_HOST/stacks" "Authorization: Bearer $ACCESS_TOKEN"
