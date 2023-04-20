#!/usr/bin/env bash

set -euo pipefail

$HTTP get "$IM_HOST/me" "Authorization: Bearer $ACCESS_TOKEN"
