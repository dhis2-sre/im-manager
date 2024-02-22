#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

INSTANCE_ID=$1

$HTTP put "$IM_HOST/instances/$INSTANCE_ID/reset" "Authorization: Bearer $ACCESS_TOKEN"
