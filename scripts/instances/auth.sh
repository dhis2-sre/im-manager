#!/usr/bin/env bash

set -euo pipefail

CACHE_FILE=./.access_token_cache_$ENVIRONMENT
touch "$CACHE_FILE"
ACCESS_TOKEN="$(cat "$CACHE_FILE" || "")"
exp=$(echo "$ACCESS_TOKEN" | jq -R 'split(".")? | .[1] | @base64d | fromjson | .exp')
NOW=$(date +%s)
if [[ -z "$exp" ]] || (( $exp < $NOW )); then
  # shellcheck disable=SC2155
  export ACCESS_TOKEN=$($HTTP --auth "$USER_EMAIL:$PASSWORD" post "$IM_HOST/tokens" | jq -r '.access_token')
  echo "$ACCESS_TOKEN" > "$CACHE_FILE"
fi
