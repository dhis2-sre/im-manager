#!/usr/bin/env bash

set -euo pipefail

echo "{}" | $HTTP --headers --auth "$ADMIN_USER_EMAIL:$ADMIN_USER_PASSWORD" post "$IM_HOST/tokens"
