#!/bin/bash

set -euo pipefail

GROUP=$1
INSTANCES=$2
NAME=$3

export STARTUP_PROBE_PERIOD_SECONDS=10

export DATABASE_ID=test-dbs-sierra-leone-dev-sql-gz
export DATABASE_SIZE=20Gi

export IMAGE_REPOSITORY=core-dev
#export IMAGE_PULL_POLICY=Always
export IMAGE_TAG=2.41

export CORE_RESOURCES_REQUESTS_CPU=500m # 250m
export CORE_RESOURCES_REQUESTS_MEMORY=2500Mi # 1500Mi


for ((i = 1; i < INSTANCES + 1; i++)); do
  # Ensure each deploy get a fresh access token
  rm -f .access_token_cache
  source ./auth.sh
  ./deploy-dhis2.sh "$GROUP" "$NAME-$i"
done
