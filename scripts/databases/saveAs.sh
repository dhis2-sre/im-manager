#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

NEW_NAME=$2
FORMAT=$3

INSTANCE_ID=$1

echo "{
  \"name\": \"$NEW_NAME\",
  \"format\": \"$FORMAT\"
}" | $HTTP post "$IM_HOST/databases/save-as/$INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
