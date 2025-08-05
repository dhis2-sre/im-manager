#!/usr/bin/env bash

set -euo pipefail

NAME=$1
NAMESPACE=$2
DESCRIPTION=$3
HOSTNAME=$4
DEPLOYABLE=${5:-false}

source ./auth.sh Admin

echo "{
  \"name\": \"$NAME\",
  \"namespace\": \"$NAMESPACE\",
  \"description\": \"$DESCRIPTION\",
  \"hostname\": \"$HOSTNAME\",
  \"deployable\": $DEPLOYABLE
}" | $HTTP post "$IM_HOST/groups" "Authorization: Bearer $ACCESS_TOKEN"
