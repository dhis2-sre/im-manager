#!/usr/bin/env bash

set -e

USER_NAME=""
NAMESPACE=""
CLUSTER_WIDE=""
POSITIONAL_ARGS=()
for arg in "$@"; do
    if [[ "$arg" == "--cluster-wide" ]]; then
        CLUSTER_WIDE="true"
    else
        POSITIONAL_ARGS+=("$arg")
    fi
done

if [[ ${#POSITIONAL_ARGS[@]} -ge 1 ]]; then
    USER_NAME="${POSITIONAL_ARGS[0]}"
fi
if [[ ${#POSITIONAL_ARGS[@]} -ge 2 ]]; then
    NAMESPACE="${POSITIONAL_ARGS[1]}"
fi

if [[ -z "$USER_NAME" ]]; then
    echo "Usage: $0 <username> [namespace] [--cluster-wide]"
    exit 1
fi

if [[ -n "$CLUSTER_WIDE" && -z "$NAMESPACE" ]]; then
    NAMESPACE="cluster-users"
fi

if [[ -z "$NAMESPACE" ]]; then
    if kubectl get clusterrolebinding "${USER_NAME}-cluster-binding" >/dev/null 2>&1; then
        NAMESPACE="cluster-users"
        CLUSTER_WIDE="true"
    else
        EXISTING_NS=$(kubectl get sa -A -l dhis2.org/user=true -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null | grep "/${USER_NAME}$" | cut -d'/' -f1 || true)
        if [[ -n "$EXISTING_NS" ]]; then
            NAMESPACE="$EXISTING_NS"
        else
            echo "ERROR: User '$USER_NAME' not found."
            exit 1
        fi
    fi
fi

if [[ "$NAMESPACE" == "cluster-users" ]]; then
    CLUSTER_WIDE="true"
fi

if [[ -z "$CLUSTER_WIDE" ]] && ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo "ERROR: Namespace '$NAMESPACE' does not exist."
    exit 1
fi

if [[ -n "$CLUSTER_WIDE" ]]; then
    echo "Revoking cluster-wide access for '$USER_NAME'..."
    kubectl delete clusterrolebinding "${USER_NAME}-cluster-binding" --ignore-not-found
    kubectl delete serviceaccount "$USER_NAME" -n "$NAMESPACE" --ignore-not-found
else
    echo "Revoking access for '$USER_NAME' in '$NAMESPACE'..."
    kubectl delete rolebinding "${USER_NAME}-binding" -n "$NAMESPACE" --ignore-not-found
    kubectl delete serviceaccount "$USER_NAME" -n "$NAMESPACE" --ignore-not-found
fi

[ -f "${USER_NAME}-config.yaml" ] && rm "${USER_NAME}-config.yaml" && echo "Removed local config file."

echo "User deleted."
