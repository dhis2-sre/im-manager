#!/usr/bin/env bash

set -euo pipefail

$HTTP get "$IM_HOST/users/health"
