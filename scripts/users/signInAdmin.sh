#!/usr/bin/env bash

set -euo pipefail

$HTTP --auth "$ADMIN_USER_EMAIL:$ADMIN_USER_PASSWORD" post "$IM_HOST/tokens"
