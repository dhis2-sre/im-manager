#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

LAST_EVENT_ID="${1:-}"

if [[ -z $LAST_EVENT_ID ]]; then
  $HTTP get "$IM_HOST/events" Cookie:accessToken="$ACCESS_TOKEN"
else
  $HTTP get "$IM_HOST/events" Cookie:accessToken="$ACCESS_TOKEN" Last-Event-ID:"$LAST_EVENT_ID"
fi
