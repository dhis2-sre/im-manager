#!/usr/bin/env bash

set -euo pipefail

REFRESH_TOKEN=$1

echo "{\"refreshToken\": \"$REFRESH_TOKEN\"}" | $HTTP post "$IM_HOST/refresh"
