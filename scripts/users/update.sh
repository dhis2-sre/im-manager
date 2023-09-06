#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh Admin

echo "{
  \"email\": \"$USER_EMAIL\",
  \"password\": \"$PASSWORD\"
}" | $HTTP put "$IM_HOST/users/$1" "Authorization: Bearer $ACCESS_TOKEN"
