#!/usr/bin/env bash

#set -euxo pipefail

HTTP="http --verify=no --check-status"

ACCESS_TOKEN=$($HTTP --auth "$USER_EMAIL:$PASSWORD" post "$INSTANCE_HOST/tokens" | jq -r '.access_token')

echo "export ACCESS_TOKEN=$ACCESS_TOKEN"
