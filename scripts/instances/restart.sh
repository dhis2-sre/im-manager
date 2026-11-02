#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

INSTANCE_ID=$1
# A component, such as dhis2, restarts only that component instead of the whole instance.
COMPONENT=${2:-}

$HTTP put "$IM_HOST/instances/$INSTANCE_ID/restart" ${COMPONENT:+"selector==$COMPONENT"} "Authorization: Bearer $ACCESS_TOKEN"
