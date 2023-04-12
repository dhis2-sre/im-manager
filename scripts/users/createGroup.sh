#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
HOSTNAME=$2

echo "{
  \"name\": \"$GROUP\",
  \"hostname\": \"$HOSTNAME\"
}" | $HTTP post "$IM_HOST/groups" "Authorization: Bearer $ACCESS_TOKEN"
