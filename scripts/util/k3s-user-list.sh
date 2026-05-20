#!/usr/bin/env bash

set -e

NAMESPACE=$1

if [[ -n "$NAMESPACE" ]]; then
    if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
        echo "ERROR: Namespace '$NAMESPACE' does not exist."
        exit 1
    fi
    echo "Users in namespace '$NAMESPACE':"
else
    echo "Managed users in cluster:"
fi

declare -A SEEN_USERS

CRB_OUTPUT=$(kubectl get clusterrolebinding -l dhis2.org/user=true -o custom-columns=NAME:.metadata.name --no-headers 2>/dev/null || true)
for name in $CRB_OUTPUT; do
    user="${name%-cluster-binding}"
    SEEN_USERS["$user"]="clusterwide"
done

if [[ -n "$NAMESPACE" ]]; then
    SA_OUTPUT=$(kubectl get sa -n "$NAMESPACE" -l dhis2.org/user=true -o jsonpath='{range .items[*]}{.metadata.namespace} {.metadata.name}{"\n"}{end}' 2>/dev/null | grep -v "^cluster-users " || true)
else
    SA_OUTPUT=$(kubectl get sa -A -l dhis2.org/user=true -o jsonpath='{range .items[*]}{.metadata.namespace} {.metadata.name}{"\n"}{end}' 2>/dev/null | grep -v "^cluster-users " || true)
fi

while read -r line; do
    [[ -z "$line" ]] && continue
    ns="${line%% *}"
    user="${line#* }"
    if [[ -z "${SEEN_USERS[$user]}" ]]; then
        SEEN_USERS["$user"]="$ns"
    fi
done <<< "$SA_OUTPUT"

if [[ ${#SEEN_USERS[@]} -eq 0 ]]; then
    echo "No users found."
    exit 0
fi

for user in "${!SEEN_USERS[@]}"; do
    scope="${SEEN_USERS[$user]}"
    if [[ "$scope" == "clusterwide" ]]; then
        echo "- $user (namespace: ALL)"
    else
        echo "- $user (namespace: $scope)"
    fi
done | sort
