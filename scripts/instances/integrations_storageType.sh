#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

key="STORAGE_TYPE"

echo "{
  \"key\": \"$key\"
}" | $HTTP post "$IM_HOST/integrations" "Authorization: Bearer $ACCESS_TOKEN"
