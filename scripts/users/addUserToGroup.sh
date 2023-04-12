#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
USER=$2

$HTTP post "$IM_HOST/groups/$GROUP/users/$USER" "Authorization: Bearer $ACCESS_TOKEN"
