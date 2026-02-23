#!/usr/bin/env bash

USER_NAME=$1
NAMESPACE=$2

# Validation: Check for arguments
if [[ -z "$USER_NAME" || -z "$NAMESPACE" ]]; then
    echo "Usage: $0 <username> <namespace>"
    exit 1
fi

# Validation: Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo "ERROR: Namespace '$NAMESPACE' does not exist."
    exit 1
fi

echo "Revoking access for '$USER_NAME' in '$NAMESPACE'..."

kubectl delete rolebinding "${USER_NAME}-binding" -n "$NAMESPACE" --ignore-not-found
kubectl delete serviceaccount "$USER_NAME" -n "$NAMESPACE" --ignore-not-found

# Clean up local file if it exists
[ -f "${USER_NAME}-config.yaml" ] && rm "${USER_NAME}-config.yaml" && echo "Removed local config file."

echo "User deleted."

