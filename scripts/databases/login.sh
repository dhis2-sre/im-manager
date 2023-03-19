#!/usr/bin/env bash

set -euo pipefail

ACCESS_TOKEN=$($HTTP --auth "$USER_EMAIL:$PASSWORD" post "$IM_HOST/tokens" | jq -r '.access_token')

echo "export ACCESS_TOKEN=$ACCESS_TOKEN"
