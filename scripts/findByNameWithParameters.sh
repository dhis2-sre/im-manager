#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2

INSTANCE_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

$HTTP get "$IM_HOST/instances/$INSTANCE_ID/parameters" "Authorization: Bearer $ACCESS_TOKEN"
