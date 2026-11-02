#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

$HTTP get "$IM_HOST/deployments/presets" "Authorization: Bearer $ACCESS_TOKEN"
