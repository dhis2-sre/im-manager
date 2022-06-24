#!/usr/bin/env bash

set -euo pipefail

PGADMIN_USERNAME=someone@something.com
PGADMIN_PASSWORD=QYfzfZTeF8pVMwECvt7jgBdk

FIRST_INSTANCE_NAME=$1
SECOND_INSTANCE_NAME=$2
GROUP_NAME=$3
STACK_NAME=pgadmin

FIRST_INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$FIRST_INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")
SECOND_INSTANCE_ID=$($HTTP --check-status "$INSTANCE_HOST/instances-name-to-id/$GROUP_NAME/$SECOND_INSTANCE_NAME" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$SECOND_INSTANCE_NAME\",
  \"groupName\": \"$GROUP_NAME\",
  \"stackName\": \"$STACK_NAME\",
  \"requiredParameters\": [
    {
      \"stackParameter\": \"PGADMIN_USERNAME\",
      \"value\": \"$PGADMIN_USERNAME\"
    },
    {
      \"stackParameter\": \"PGADMIN_PASSWORD\",
      \"value\": \"$PGADMIN_PASSWORD\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances/$FIRST_INSTANCE_ID/link/$SECOND_INSTANCE_ID" "Authorization: Bearer $ACCESS_TOKEN"
