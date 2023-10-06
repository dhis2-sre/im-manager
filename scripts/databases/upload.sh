#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
FILE=$2
PREFIX=${3:-""}

#$HTTP --ignore-stdin --form post "$IM_HOST/databases" "group=$GROUP" "database@$FILE" "Authorization: Bearer $ACCESS_TOKEN"
curl --fail --progress-bar -H "Authorization: $ACCESS_TOKEN" -F "group=$GROUP" -F "prefix=$PREFIX" -F "database=@$FILE" -L "$IM_HOST/databases" | cat
