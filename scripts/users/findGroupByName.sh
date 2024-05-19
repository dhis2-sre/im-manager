#!/usr/bin/env bash

set -euo pipefail

NAME=$1

source ./auth.sh

$HTTP get "$IM_HOST/groups/$NAME" "Authorization: Bearer $ACCESS_TOKEN"
