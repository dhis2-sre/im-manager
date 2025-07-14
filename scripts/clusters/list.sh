#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

$HTTP get "$IM_HOST/clusters" "Authorization: Bearer $ACCESS_TOKEN"
