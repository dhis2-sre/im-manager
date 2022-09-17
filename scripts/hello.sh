#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2

function int_handler {
  ./destroy.sh $GROUP $NAME
  trap - EXIT
  exit
}

trap int_handler INT
trap int_handler EXIT

./deploy-whoami.sh $GROUP $NAME

./findByName.sh $GROUP $NAME

sleep 5

./logs.sh $GROUP $NAME
