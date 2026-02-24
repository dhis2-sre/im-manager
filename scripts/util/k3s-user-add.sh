#!/usr/bin/env bash

set -e

USER_NAME=$1
NAMESPACE=$2
DURATION=${3:-8760h}
OUTPUT_FILE="${USER_NAME}-config.yaml"

if [[ -z "$USER_NAME" || -z "$NAMESPACE" ]]; then
    echo "Usage: $0 <username> <namespace> [duration]"
    exit 1
fi

if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo "ERROR: Namespace '$NAMESPACE' does not exist."
    exit 1
fi

echo "Creating access for '$USER_NAME' in '$NAMESPACE'..."

kubectl create serviceaccount "$USER_NAME" -n "$NAMESPACE"

cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${USER_NAME}-binding
  namespace: ${NAMESPACE}
subjects:
- kind: ServiceAccount
  name: ${USER_NAME}
  namespace: ${NAMESPACE}
roleRef:
  kind: ClusterRole
  name: edit
  apiGroup: rbac.authorization.k8s.io
EOF

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

echo "Success! Config saved to: $OUTPUT_FILE"
