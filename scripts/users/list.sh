#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh Admin

$HTTP get "$IM_HOST/users" "Authorization: Bearer $ACCESS_TOKEN"
