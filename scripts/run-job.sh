#!/usr/bin/env bash

set -euo pipefail

NAME=$1
GROUP_NAME=$2

export ACCESS_TOKEN="" && eval "$(./login.sh)" && echo "$ACCESS_TOKEN"

./create-job.sh "$NAME" "$GROUP_NAME"
./deploy-job.sh "$NAME" "$GROUP_NAME"
./logs.sh "$NAME" "$GROUP_NAME"
