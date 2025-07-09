#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

ID=${1:-}

$HTTP delete "$IM_HOST/clusters/$ID" "Authorization: Bearer $ACCESS_TOKEN" 