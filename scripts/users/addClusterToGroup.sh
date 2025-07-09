#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
CLUSTER_ID=$2

source ./auth.sh Admin

$HTTP post "$IM_HOST/groups/$GROUP/clusters/$CLUSTER_ID" "Authorization: Bearer $ACCESS_TOKEN" 