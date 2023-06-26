#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

$HTTP get "$IM_HOST/groups" "Authorization: Bearer $ACCESS_TOKEN"
