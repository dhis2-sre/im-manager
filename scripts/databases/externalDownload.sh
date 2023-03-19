#!/usr/bin/env bash

set -euo pipefail

UUID=$1

curl -L "$IM_HOST/databases/external/$UUID" | cat
