#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
SELECTOR=${3:-""}

INSTANCE_ID=$($HTTP get "$IM_HOST/instances-name-to-id/$GROUP/$NAME" "Authorization: Bearer $ACCESS_TOKEN")

curl -N "$IM_HOST/instances/$INSTANCE_ID/logs?selector=$SELECTOR" -H "Authorization: Bearer $ACCESS_TOKEN"
