#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
PLAIN_TEXT_CONFIG_FILE=$2

ENCRYPTED_CONFIG_FILE=$(mktemp)

sops -e "$PLAIN_TEXT_CONFIG_FILE" > $ENCRYPTED_CONFIG_FILE

$HTTP --ignore-stdin --form post "$IM_HOST/groups/$GROUP/cluster-configuration" "kubernetesConfiguration@$ENCRYPTED_CONFIG_FILE" "Authorization: Bearer $ACCESS_TOKEN"

rm $ENCRYPTED_CONFIG_FILE
