#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh Admin

DEPLOYABLE=${1:-false}

$HTTP get "$IM_HOST/groups?deployable=$DEPLOYABLE" "Authorization: Bearer $ACCESS_TOKEN"
