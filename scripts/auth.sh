#!/usr/bin/env bash

set -euo pipefail

IM_USER_TYPE="${1:-}"

touch ./.access_token_cache
ACCESS_TOKEN="$(cat ./.access_token_cache || "")"
exp=$(echo "$ACCESS_TOKEN" | jq -R 'split(".")? | .[1] | @base64d | fromjson | .exp')
NOW=$(date +%s)
RESPONSE=""
if [[ -z "$exp" ]] || (( exp < NOW )); then
  # shellcheck disable=SC2155
  if [[ -z "$IM_USER_TYPE" ]]; then
    RESPONSE=$(echo "{}" | $HTTP --headers --auth "$USER_EMAIL:$PASSWORD" post "$IM_HOST/tokens")
  else
    RESPONSE=$(echo "{}" | $HTTP --headers --auth "$ADMIN_USER_EMAIL:$ADMIN_USER_PASSWORD" post "$IM_HOST/tokens")
  fi
  # shellcheck disable=SC2155
  export ACCESS_TOKEN=$(echo "$RESPONSE" | grep Set-Cookie | grep accessToken | sed s/Set-Cookie:\ accessToken=// | cut -f1 -d ";")
  echo "$ACCESS_TOKEN" > ./.access_token_cache
fi
