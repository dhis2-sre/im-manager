#!/usr/bin/env bash

set -euo pipefail

IM_USER_TYPE="${1:-}"

TOKEN_CACHE_FILE=./.access_token_cache_$IM_USER_TYPE
touch "$TOKEN_CACHE_FILE"

ACCESS_TOKEN="$(cat "$TOKEN_CACHE_FILE" || echo "")"
exp=$(echo "$ACCESS_TOKEN" | jq -R 'split(".")? | .[1] | @base64d | fromjson | .exp')
NOW=$(date +%s)
if [[ -z "$exp" ]] || (( exp < NOW )); then
  if [ "$IM_USER_TYPE" == "Admin" ]; then
    USER_EMAIL=$ADMIN_USER_EMAIL
    PASSWORD=$ADMIN_USER_PASSWORD
  fi
  RESPONSE=$(echo "{}" | $HTTP --headers --auth "$USER_EMAIL:$PASSWORD" post "$IM_HOST/tokens")
  # shellcheck disable=SC2155
  export ACCESS_TOKEN=$(echo "$RESPONSE" | grep Set-Cookie | grep accessToken | sed s/Set-Cookie:\ accessToken=// | cut -f1 -d ";")
  echo "$ACCESS_TOKEN" > "$TOKEN_CACHE_FILE"
fi
