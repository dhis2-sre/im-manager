#!/usr/bin/env bash

set -e

USER_NAME=""
NAMESPACE=""
DURATION="8760h"
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
if [[ ${#POSITIONAL_ARGS[@]} -ge 3 ]]; then
    DURATION="${POSITIONAL_ARGS[2]}"
fi

OUTPUT_FILE="${USER_NAME}-config.yaml"

if [[ -z "$USER_NAME" ]]; then
    echo "Usage: $0 <username> [namespace] [--cluster-wide] [duration]"
    exit 1
fi

if [[ -n "$CLUSTER_WIDE" && -z "$NAMESPACE" ]]; then
    NAMESPACE="cluster-users"
fi

if [[ -z "$NAMESPACE" ]]; then
    echo "Usage: $0 <username> <namespace> [duration]"
    exit 1
fi

if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo "Creating namespace '$NAMESPACE'..."
    kubectl create namespace "$NAMESPACE"
fi

if kubectl get clusterrolebinding "${USER_NAME}-cluster-binding" >/dev/null 2>&1; then
    echo "ERROR: User '$USER_NAME' already exists with cluster-wide access."
    exit 1
fi

EXISTING_NS=$(kubectl get sa -A -l dhis2.org/user=true -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name --no-headers 2>/dev/null | grep " ${USER_NAME}$" | awk '{print $1}' || true)
if [[ -n "$EXISTING_NS" ]]; then
    echo "ERROR: User '$USER_NAME' already exists with namespace-scoped access in namespace '$EXISTING_NS'."
    exit 1
fi

echo "Creating access for '$USER_NAME' in '$NAMESPACE'..."

if [[ -n "$CLUSTER_WIDE" ]]; then
    echo "Note: Cluster-wide access grants access to ALL namespaces."
    kubectl create serviceaccount "$USER_NAME" -n "$NAMESPACE"
    kubectl label serviceaccount "$USER_NAME" -n "$NAMESPACE" dhis2.org/user="true"

    BINDING_KIND="ClusterRoleBinding"
    BINDING_NAME="${USER_NAME}-cluster-binding"
    cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ${BINDING_KIND}
metadata:
  name: ${BINDING_NAME}
  labels:
    dhis2.org/user: "true"
subjects:
- kind: ServiceAccount
  name: ${USER_NAME}
  namespace: ${NAMESPACE}
roleRef:
  kind: ClusterRole
  name: edit
  apiGroup: rbac.authorization.k8s.io
EOF
else
    kubectl create serviceaccount "$USER_NAME" -n "$NAMESPACE"
    kubectl label serviceaccount "$USER_NAME" -n "$NAMESPACE" dhis2.org/user="true"

    cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${USER_NAME}-binding
  namespace: ${NAMESPACE}
  labels:
    dhis2.org/user: "true"
subjects:
- kind: ServiceAccount
  name: ${USER_NAME}
  namespace: ${NAMESPACE}
roleRef:
  kind: ClusterRole
  name: edit
  apiGroup: rbac.authorization.k8s.io
EOF
fi

TOKEN=$(kubectl create token "$USER_NAME" -n "$NAMESPACE" --duration="$DURATION")

# Extract from the current cluster context
CLUSTER_NAME=$(kubectl config view --minify -o jsonpath='{.clusters[0].name}')
SERVER_URL=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
CA_DATA=$(kubectl config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

cat <<EOF > "$OUTPUT_FILE"
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CA_DATA}
    server: ${SERVER_URL}
  name: ${CLUSTER_NAME}
contexts:
- context:
    cluster: ${CLUSTER_NAME}
    namespace: ${NAMESPACE}
    user: ${USER_NAME}
  name: ${USER_NAME}-context
current-context: ${USER_NAME}-context
users:
- name: ${USER_NAME}
  user:
    token: ${TOKEN}
EOF

if [[ -n "$CLUSTER_WIDE" ]]; then
    echo "Success! Config saved to: $OUTPUT_FILE (cluster-wide access)"
else
    echo "Success! Config saved to: $OUTPUT_FILE"
fi
