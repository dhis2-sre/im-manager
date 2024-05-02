#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2

$HTTP get "$IM_HOST/deployments" "Authorization: Bearer $ACCESS_TOKEN" | jq -r ".[] | select(.name==\"$GROUP\") | .deployments[] | select(.name==\"$NAME\")"
