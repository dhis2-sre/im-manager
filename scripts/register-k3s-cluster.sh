#!/bin/sh
set -e

echo "Waiting for /output/kubeconfig.yaml..."
until [ -f /output/kubeconfig.yaml ]; do sleep 2; done

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
    "hostname": "'${GROUP_HOSTNAME}'",
    "namespace": "'${GROUP_NAMESPACE}'",
    "description": "'${GROUP_NAME}' group",
    "deployable": true
  }'

CLUSTER_RESPONSE=$(curl --request POST "http://${IM_HOSTNAME}/clusters" \
  --header "Authorization: Bearer ${ACCESS_TOKEN}" \
  --form "name=${GROUP_NAME}" \
  --form "description=K3s ${GROUP_NAME} cluster" \
  --form "kubernetesConfiguration=@/output/kubeconfig.yaml")
CLUSTER_ID=$(echo "${CLUSTER_RESPONSE}" | grep -o '"id":[0-9]*' | sed 's/"id"://' | tr -d '"')

curl --request POST "http://${IM_HOSTNAME}/groups/${GROUP_NAME}/clusters/${CLUSTER_ID}" --header "Authorization: Bearer ${ACCESS_TOKEN}"

echo "Cluster added to the \"${GROUP_NAME}\" group."
