#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

INSTANCE_ID=$1

$HTTP get "$IM_HOST/instances/$INSTANCE_ID/status" "Authorization: Bearer $ACCESS_TOKEN"
