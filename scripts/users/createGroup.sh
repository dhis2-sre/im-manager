#!/usr/bin/env bash

set -euo pipefail

NAME=$1
DESCRIPTION=$2
HOSTNAME=$2
DEPLOYABLE=${3:-false}

source ./auth.sh Admin

echo "{
  \"name\": \"$NAME\",
  \"description\": \"$HOSTNAME\",
  \"hostname\": \"$HOSTNAME\",
  \"deployable\": $DEPLOYABLE
}" | $HTTP post "$IM_HOST/groups" "Authorization: Bearer $ACCESS_TOKEN"
