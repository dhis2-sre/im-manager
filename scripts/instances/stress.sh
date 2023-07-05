#!/bin/bash

set -euo pipefail

source ./auth.sh

GROUP=$1
INSTANCES=$2
NAME=$3

for ((i = 0; i < INSTANCES; i++)); do
  ./deploy-whoami.sh "$GROUP" "$NAME-$i" &
done

# shellcheck disable=SC2046
wait $(jobs -p)
