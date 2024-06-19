#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

$HTTP get "$IM_HOST/events" Cookie:accessToken=$ACCESS_TOKEN
