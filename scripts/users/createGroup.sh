#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
HOSTNAME=$2
DEPLOYABLE=${3:-false}

echo "{
  \"name\": \"$GROUP\",
  \"hostname\": \"$HOSTNAME\",
  \"deployable\": $DEPLOYABLE
}" | $HTTP post "$IM_HOST/groups" "Authorization: Bearer $ACCESS_TOKEN"
