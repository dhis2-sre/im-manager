#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
shift
shift
DESCRIPTION=${*:-""}

echo "{
  \"name\": \"$NAME\",
  \"group\": \"$GROUP\",
  \"description\": \"$DESCRIPTION\",
}" | $HTTP post "$IM_HOST/deployments" "Authorization: Bearer $ACCESS_TOKEN"
