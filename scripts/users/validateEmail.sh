#!/usr/bin/env bash

set -euo pipefail

TOKEN=$1

echo "{
  \"token\": \"$TOKEN\"
}" | $HTTP post "$IM_HOST/users/validate"
