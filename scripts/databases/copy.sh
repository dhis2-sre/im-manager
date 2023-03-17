#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

SOURCE=$1
GROUP=$2
NAME=$3

echo "{
  \"name\": \"$NAME\",
  \"group\": \"$GROUP\"
}" | $HTTP post "$IM_HOST/databases/$SOURCE/copy" "Authorization: Bearer $ACCESS_TOKEN"
