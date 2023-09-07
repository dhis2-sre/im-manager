#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

ID=$1

$HTTP get "$IM_HOST/deployments/$ID" "Authorization: Bearer $ACCESS_TOKEN"
