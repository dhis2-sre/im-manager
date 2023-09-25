#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENTS=$*

echo "Destroying deployment(s): $DEPLOYMENTS"

delete(){
  $HTTP delete "$IM_HOST/deployments/$DEPLOYMENT_ID" "Authorization: Bearer $ACCESS_TOKEN"
}

for DEPLOYMENT_ID in $DEPLOYMENTS; do
  delete "$DEPLOYMENT_ID" &
done

# shellcheck disable=SC2046
wait $(jobs -p)
