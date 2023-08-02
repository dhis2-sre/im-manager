#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
REPLICA_COUNT=$3

INSTANCE_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"parameters\": [
    {
       \"name\": \"REPLICA_COUNT\",
       \"value\": \"$REPLICA_COUNT\"
    }
  ]
}" | $HTTP put "$IM_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
