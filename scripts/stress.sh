#!/bin/bash

set -euo pipefail

GROUP=$1
INSTANCES=$2
NAME=$3

for ((i = 0; i < $INSTANCES; i++)); do
  ./deploy-whoami.sh $GROUP $NAME-$i &
done

wait $(jobs -p)
