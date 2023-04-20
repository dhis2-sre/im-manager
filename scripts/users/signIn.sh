#!/usr/bin/env bash

set -euo pipefail

$HTTP --auth "$USER_EMAIL:$PASSWORD" post "$IM_HOST/tokens"
