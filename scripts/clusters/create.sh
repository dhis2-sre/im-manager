#!/usr/bin/env bash

set -euo pipefail

if [ $# -ne 3 ]; then
    echo "Usage: $0 <cluster-name> <description> <plain-text-config-file>"
    exit 1
fi

source ./auth.sh

NAME=$1
DESCRIPTION=$2
PLAIN_TEXT_CONFIG_FILE=$3

ENCRYPTED_CONFIG_FILE=$(mktemp)

sops --input-type yaml --output-type yaml --encrypt "$PLAIN_TEXT_CONFIG_FILE" > $ENCRYPTED_CONFIG_FILE

$HTTP --form post "$IM_HOST/clusters" \
  "Authorization: Bearer $ACCESS_TOKEN" \
  name="$NAME" \
  description="$DESCRIPTION" \
  kubernetesConfiguration@"$ENCRYPTED_CONFIG_FILE"
