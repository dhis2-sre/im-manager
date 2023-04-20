#!/usr/bin/env bash

set -euo pipefail

$HTTP delete "$IM_HOST/users" "Authorization: Bearer $ACCESS_TOKEN"
