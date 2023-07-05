#!/usr/bin/env bash

set -euo pipefail

touch ./.access_token_cache
ACCESS_TOKEN="$(cat ./.access_token_cache || "")"
exp=$(echo "$ACCESS_TOKEN" | jq -R 'split(".")? | .[1] | @base64d | fromjson | .exp')
NOW=$(date +%s)
if [[ -z "$exp" ]] || (( exp < NOW )); then
  # shellcheck disable=SC2155
  export ACCESS_TOKEN=$($HTTP --auth "$USER_EMAIL:$PASSWORD" post "$IM_HOST/tokens" | jq -r '.accessToken')
  echo "$ACCESS_TOKEN" > ./.access_token_cache
fi
