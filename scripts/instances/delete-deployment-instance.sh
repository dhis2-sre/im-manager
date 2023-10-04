#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DEPLOYMENT_ID=$1
INSTANCE_ID=$2

$HTTP delete "$IM_HOST/deployments/$DEPLOYMENT_ID/instance/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
