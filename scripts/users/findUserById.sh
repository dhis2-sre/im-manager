#!/usr/bin/env bash

set -euo pipefail

USER_ID=$1

source ./auth.sh

$HTTP get "$IM_HOST/users/$USER_ID" "Authorization: Bearer $ACCESS_TOKEN"
