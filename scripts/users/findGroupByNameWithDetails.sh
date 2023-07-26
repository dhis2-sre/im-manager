#!/usr/bin/env bash

set -euo pipefail

NAME=$1

$HTTP get "$IM_HOST/groups/$NAME/details" "Authorization: Bearer $ACCESS_TOKEN"
