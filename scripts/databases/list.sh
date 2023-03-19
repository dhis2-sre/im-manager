#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

$HTTP get "$IM_HOST/databases" "Authorization: Bearer $ACCESS_TOKEN"
