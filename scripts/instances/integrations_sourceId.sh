#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

key="SOURCE_ID"

echo "{
  \"key\": \"$key\"
}" | $HTTP post "$IM_HOST/integrations" "Authorization: Bearer $ACCESS_TOKEN"
