#!/usr/bin/env bash

set -euo pipefail

$HTTP delete "$IM_HOST/users/$1" "Authorization: Bearer $ACCESS_TOKEN"
