#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

$HTTP get "$IM_HOST/me" "Authorization: Bearer $ACCESS_TOKEN"
