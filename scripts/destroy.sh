#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
shift
INSTANCES=$*

echo "Group: $GROUP"
echo "Instance(s): $INSTANCES"

delete(){
  INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP/$1" "Authorization: Bearer $ACCESS_TOKEN")
  $HTTP delete "$INSTANCE_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
}

for INSTANCE in $INSTANCES; do
  delete $INSTANCE &
done

wait $(jobs -p)
