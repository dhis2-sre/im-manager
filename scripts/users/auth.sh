#!/usr/bin/env bash

set -euo pipefail

IM_USER_TYPE="${1:-}"

touch ./.access_token_cache
ACCESS_TOKEN="$(cat ./.access_token_cache || "")"
exp=$(echo "$ACCESS_TOKEN" | jq -R 'split(".")? | .[1] | @base64d | fromjson | .exp')
NOW=$(date +%s)
#if $IM_USER_TYPE != "" and token doesn't match
if [[ -z "$exp" ]] || (( exp < NOW )); then
  # shellcheck disable=SC2155
  export ACCESS_TOKEN=$(./signIn$IM_USER_TYPE.sh | jq -r '.accessToken')
  echo "$ACCESS_TOKEN" > ./.access_token_cache
fi
