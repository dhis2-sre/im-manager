#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

INSTANCE_ID=$1

echo "{ \"name\": \"name\" }" | $HTTP post "$IM_HOST/instances/$INSTANCE_ID/backup" "Authorization: Bearer $ACCESS_TOKEN"
