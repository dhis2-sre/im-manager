#!/usr/bin/env bash

set -euo pipefail

$HTTP get "$IM_HOST/users" "Authorization: Bearer $ACCESS_TOKEN"
