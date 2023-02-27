#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

STACK=$1

$HTTP get "$IM_HOST/stacks/$STACK" "Authorization: Bearer $ACCESS_TOKEN"
