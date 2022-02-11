#!/usr/bin/env bash

set -euo pipefail

$HTTP "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
