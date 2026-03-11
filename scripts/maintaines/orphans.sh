#!/usr/bin/env bash

set -euo pipefail
declare -a orphan_releases=()

# Get all namespaces
namespaces=($(kubectl get ns -o jsonpath='{.items[*].metadata.name}'))

declare -A has_base
declare -A has_database
declare -A has_minio
declare -A prefixes

echo "Gathering Helm releases from all namespaces..."

for ns in "${namespaces[@]}"; do
  # List releases in this namespace; suppress errors if no releases
  mapfile -t releases < <(helm list -n "$ns" -o json 2>/dev/null | jq -r '.[].name' || true)

  for name in "${releases[@]}"; do
    if [[ "$name" =~ ^(.+)-database$ ]]; then
      prefix="${BASH_REMATCH[1]}"
      has_database["$ns:$prefix"]=1
      prefixes["$ns:$prefix"]=1
    elif [[ "$name" =~ ^(.+)-minio$ ]]; then
      prefix="${BASH_REMATCH[1]}"
      has_minio["$ns:$prefix"]=1
      prefixes["$ns:$prefix"]=1
    else
      has_base["$ns:$name"]=1
    fi
  done
done

echo "== Checking for orphaned Helm releases (database/minio without base) =="

for key in "${!prefixes[@]}"; do
  if [[ -z "${has_base[$key]+x}" ]]; then
    ns="${key%%:*}"
    prefix="${key#*:}"

    if [[ -n "${has_database[$key]+x}" ]]; then
      orphan_releases+=("$ns:${prefix}-database")
    fi

    if [[ -n "${has_minio[$key]+x}" ]]; then
      orphan_releases+=("$ns:${prefix}-minio")
    fi
  fi
done

if [[ ${#orphan_releases[@]} -eq 0 ]]; then
  echo "No orphaned releases found."
  exit 0
fi

echo "Found the following orphaned releases:"
for release in "${orphan_releases[@]}"; do
  ns="${release%%:*}"
  name="${release#*:}"
  echo " - $name (namespace: $ns)"
done

read -rp "Delete all these orphaned releases? [y/N] " confirm_all

if [[ "$confirm_all" =~ ^[Yy]$ ]]; then
  for release in "${orphan_releases[@]}"; do
    ns="${release%%:*}"
    name="${release#*:}"
    echo "Deleting $name in namespace $ns..."
    if helm uninstall "$name" -n "$ns"; then
      echo "✅ Deleted $name"
    else
      echo "⚠️ Failed to delete $name"
    fi
  done
else
  echo "No releases deleted."
fi
