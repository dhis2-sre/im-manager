#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP="${1:-${GROUP:?Set GROUP (e.g. whoami)}}"

./findDeployments.sh > .deployments.json
# Get deployment id and name per deployment (list endpoint may not include all instances)
deployments_list=$(jq -r --arg g "$GROUP" '.[] | select(.name==$g) | .deployments[] | "\(.id) \(.name)"' .deployments.json)
if [[ -z "$deployments_list" ]]; then
  echo "No deployments found for group $GROUP"
  exit 0
fi
while IFS= read -r line; do
  # Re-validate/refresh token so long runs don't hit 401 (token is checked and refreshed if expired).
  source ./auth.sh
  dep_id="${line%% *}"
  dep_name="${line#* }"
  # Fetch full deployment by ID so we get every instance (dhis2-db, minio, dhis2-core, pgadmin, etc.)
  # Use </dev/null so $HTTP does not consume the loop's stdin (remaining deployment lines).
  dep_json=$($HTTP get "$IM_HOST/deployments/$dep_id" "Authorization: Bearer $ACCESS_TOKEN" < /dev/null)
  instances=$(echo "$dep_json" | jq -r '.instances[]? | "\(.id) \(.stackName)"')
  if [[ -z "$instances" ]]; then
    echo "Deployment \"$dep_name\" has no instances, skipping."
    continue
  fi
  echo "=== Resetting deployment: $dep_name ==="
  while read -r inst_id stack_name; do
    echo "  Resetting instance $inst_id ($stack_name) ..."
    ./reset.sh "$inst_id" < /dev/null
  done <<< "$instances"
done <<< "$deployments_list"
echo "Done resetting all deployments in group $GROUP"
