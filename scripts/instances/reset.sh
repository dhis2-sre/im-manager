#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

INSTANCE_ID=$1
# A component, such as db, resets only that component's data instead of the whole instance.
COMPONENT=${2:-}

$HTTP put "$IM_HOST/instances/$INSTANCE_ID/reset" ${COMPONENT:+"selector==$COMPONENT"} "Authorization: Bearer $ACCESS_TOKEN"
