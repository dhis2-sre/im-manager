#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
shift
INSTANCES=$*

echo "Group: $GROUP"
echo "Instance(s): $INSTANCES"

delete(){
  INSTANCE_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$1" "Authorization: Bearer $ACCESS_TOKEN")
  $HTTP delete "$IM_HOST/instances/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
}

for INSTANCE in $INSTANCES; do
  delete "$INSTANCE" &
done

# shellcheck disable=SC2046
wait $(jobs -p)
