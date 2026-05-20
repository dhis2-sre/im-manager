#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

CLUSTER=$1

$HTTP get "$IM_HOST/clusters/$CLUSTER" "Authorization: Bearer $ACCESS_TOKEN"
