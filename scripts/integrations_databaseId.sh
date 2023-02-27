#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

key="DATABASE_ID"

echo "{
  \"key\": \"$key\"
}" | $HTTP post "$IM_HOST/integrations" "Authorization: Bearer $ACCESS_TOKEN"
