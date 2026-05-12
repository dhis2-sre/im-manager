#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

ID=${1:-}
NAME=${2:-}
DESCRIPTION=${3:-}
KUBECONFIG_FILE=${4:-}

$HTTP --form put "$IM_HOST/clusters/$ID" \
  "Authorization: Bearer $ACCESS_TOKEN" \
  name="$NAME" \
  description="$DESCRIPTION" \
  kubernetesConfiguration@"$KUBECONFIG_FILE"
