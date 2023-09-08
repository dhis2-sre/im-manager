#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

ID=$1

$HTTP post "$IM_HOST/deployments/$ID/deploy" "Authorization: Bearer $ACCESS_TOKEN"
