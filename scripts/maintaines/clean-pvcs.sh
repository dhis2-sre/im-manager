#!/usr/bin/env bash

set -euo pipefail

echo "Fetching all PVCs (namespace | pvc-name | status)..."
all_pvcs=$(kubectl get pvc --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}{"|"}{.metadata.name}{"|"}{.status.phase}{"\n"}{end}')
echo "$all_pvcs" | column -t -s '|'

echo
echo "Finding PVCs that are Bound but NOT used by any Pod..."

# PVCs currently used by pods in format: ns|pvc
used_pvcs=$(kubectl get pods --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}{"|"}{.spec.volumes[*].persistentVolumeClaim.claimName}{"\n"}{end}' \
          | grep -v '^|$' | sort -u)

# All PVCs that are Bound (ns|pvc)
bound_pvcs=$(echo "$all_pvcs" | grep '|Bound$' | awk -F'|' '{print $1 "|" $2}' | sort -u)

# Find Bound PVCs NOT in used_pvcs (unused)
unused_pvcs=$(comm -23 <(echo "$bound_pvcs") <(echo "$used_pvcs"))

if [[ -z "$unused_pvcs" ]]; then
  echo "No Bound PVCs unused by any Pod found."
  exit 0
fi

echo
echo "Unused Bound PVCs (namespace | pvc-name):"
echo "$unused_pvcs" | column -t -s '|'

echo
read -rp "Do you want to delete these PVCs? (y/N) " confirm

if [[ "$confirm" =~ ^[Yy]$ ]]; then
  echo "Deleting unused PVCs..."
  echo "$unused_pvcs" | while IFS='|' read -r ns pvc; do
    kubectl delete pvc "$pvc" -n "$ns"
  done
else
  echo "Aborted. No PVCs deleted."
fi
