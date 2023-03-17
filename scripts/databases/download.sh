#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DATABASE=$1

curl -H "Authorization: $ACCESS_TOKEN" -L "$IM_HOST/databases/$DATABASE/download" | cat
