#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

key="IMAGE_TAG"
organization=$1
repository=$2

echo "{
  \"key\": \"$key\",
  \"payload\": {
    \"organization\": \"$organization\",
    \"repository\": \"$repository\"
  }
}" | $HTTP post "$INSTANCE_HOST/integrations" "Authorization: Bearer $ACCESS_TOKEN"
