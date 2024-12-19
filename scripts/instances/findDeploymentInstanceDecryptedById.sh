#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

ID=$1

$HTTP get "$IM_HOST/instances/$ID/decrypted-details" "Authorization: Bearer $ACCESS_TOKEN"
