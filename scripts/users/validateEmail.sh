#!/usr/bin/env bash

set -euo pipefail

TOKEN=$1

$HTTP get "$IM_HOST/users/validate/$TOKEN"
