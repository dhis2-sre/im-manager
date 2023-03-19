#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DATABASE_ID=$1
INSTANCE_ID=$2

echo "{
  \"instanceId\": $INSTANCE_ID
}" | $HTTP post "$IM_HOST/databases/$DATABASE_ID/lock" "Authorization: Bearer $ACCESS_TOKEN"
