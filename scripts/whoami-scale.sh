#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
NAME=$2
REPLICA_COUNT=$3

INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"optionalParameters\": [
    {
       \"name\": \"REPLICA_COUNT\",
       \"value\": \"$REPLICA_COUNT\"
    }
  ]
}" | $HTTP put "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
