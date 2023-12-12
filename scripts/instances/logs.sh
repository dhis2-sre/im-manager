#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

INSTANCE_ID=$1
SELECTOR=${2:-""}

curl -N "$IM_HOST/instances/$INSTANCE_ID/logs?selector=$SELECTOR" -H "Authorization: Bearer $ACCESS_TOKEN"
