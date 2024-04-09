#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

INSTANCE_ID=$1
$HTTP post "$IM_HOST/databases/save/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
