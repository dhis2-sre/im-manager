#!/usr/bin/env bash

set -euo pipefail

GROUP=$1

source ./auth.sh

$HTTP get "$IM_HOST/groups/$GROUP/resources" "Authorization: Bearer $ACCESS_TOKEN"
