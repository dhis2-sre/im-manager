#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DATABASE=$1
NAME=$2

echo "{
  \"name\": \"$NAME\"
}" | $HTTP put "$IM_HOST/databases/$DATABASE" "Authorization: Bearer $ACCESS_TOKEN"
