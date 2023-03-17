#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DATABASE=$1
EXPIRATION=$2

echo "{
  \"expiration\": \"$EXPIRATION\"
}" | $HTTP post "$IM_HOST/databases/$DATABASE/external" "Authorization: Bearer $ACCESS_TOKEN"
