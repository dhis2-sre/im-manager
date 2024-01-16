#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

$HTTP get "$IM_HOST/subscribe?token=$ACCESS_TOKEN"
