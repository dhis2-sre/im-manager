#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

key="IMAGE_REPOSITORY"
organization=$1

echo "{
  \"key\": \"$key\",
  \"payload\": {
    \"organization\": \"$organization\"
  }
}" | $HTTP post "$INSTANCE_HOST/integrations" "Authorization: Bearer $ACCESS_TOKEN"
