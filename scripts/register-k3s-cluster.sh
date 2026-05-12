#!/bin/sh
set -e

echo "Waiting for /output/kubeconfig.yaml..."
until [ -f /output/kubeconfig.yaml ]; do sleep 2; done

echo "Waiting for IM at http://${IM_HOSTNAME}..."
until curl --silent --output /dev/null --connect-timeout 2 "http://${IM_HOSTNAME}/health"; do sleep 2; done

ACCESS_TOKEN=$(curl --silent --dump-header - --request POST "http://${IM_HOSTNAME}/tokens" \
  --user "${IM_ADMIN_EMAIL}:${IM_ADMIN_PASSWORD}" \
  --header "Content-Type: application/json" \
  | grep "Set-Cookie:" | grep "accessToken" \
  | sed 's/Set-Cookie: accessToken=//' | cut -f 1 -d ";")

curl --request POST "http://${IM_HOSTNAME}/groups" \
  --header "Authorization: Bearer ${ACCESS_TOKEN}" \
  --header "Content-Type: application/json" \
  --data-raw '{
    "name": "'${GROUP_NAME}'",
    "hostname": "'${GROUP_HOSTNAME}.im.127-0-0-1.nip.io'",
    "namespace": "'${GROUP_NAMESPACE}'",
    "description": "'${GROUP_NAME}' group",
    "deployable": true
  }'

KUBECONFIG=$(mktemp)
sed "s|127.0.0.1:6443|k3s-${GROUP_NAME}:6443|" /output/kubeconfig.yaml > "${KUBECONFIG}"

CLUSTER_RESPONSE=$(curl --request POST "http://${IM_HOSTNAME}/clusters" \
  --header "Authorization: Bearer ${ACCESS_TOKEN}" \
  --form "name=${GROUP_NAME}" \
  --form "description=K3s ${GROUP_NAME} cluster" \
  --form "kubernetesConfiguration=@${KUBECONFIG}")
CLUSTER_ID=$(echo "${CLUSTER_RESPONSE}" | grep -o '"id":[0-9]*' | sed 's/"id"://' | tr -d '"')

curl --request POST "http://${IM_HOSTNAME}/groups/${GROUP_NAME}/clusters/${CLUSTER_ID}" --header "Authorization: Bearer ${ACCESS_TOKEN}"

echo "Cluster added to the \"${GROUP_NAME}\" group."
