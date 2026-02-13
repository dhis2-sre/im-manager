#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP="${1:-${GROUP:?Set GROUP (e.g. whoami)}}"

./findDeployments.sh > .deployments.json
deployments_json=$(jq -c --arg g "$GROUP" '.[] | select(.name==$g) | .deployments[] | {name: .name, instances: [.instances[]? | {id: .id, stackName: .stackName}]}' .deployments.json)
if [[ -z "$deployments_json" ]]; then
  echo "No deployments found for group $GROUP"
  exit 0
fi
while IFS= read -r dep; do
  dep_name=$(jq -r '.name' <<< "$dep")
  instances=$(jq -r '.instances[]? | "\(.id) \(.stackName)"' <<< "$dep")
  if [[ -z "$instances" ]]; then
    echo "Deployment \"$dep_name\" has no instances, skipping."
    continue
  fi
  echo "=== Resetting deployment: $dep_name ==="
  while read -r inst_id stack_name; do
    echo "  Resetting instance $inst_id ($stack_name) ..."
    ./reset.sh "$inst_id"
  done <<< "$instances"
done <<< "$deployments_json"
echo "Done resetting all deployments in group $GROUP"
