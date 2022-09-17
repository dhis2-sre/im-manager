#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

PGADMIN_USERNAME=${PGADMIN_USERNAME:-someone@something.com}
RANDOM_PASSWORD=$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c32)
PGADMIN_PASSWORD=${PGADMIN_PASSWORD:-$RANDOM_PASSWORD}

echo "pgAdmin username: $PGADMIN_USERNAME"
echo "pgAdmin password: $PGADMIN_PASSWORD"

STACK=pgadmin

GROUP=$1
SOURCE_INSTANCE=$2
DESTINATION_INSTANCE=$3

INSTANCE_TTL=${INSTANCE_TTL:-""}

SOURCE_INSTANCE_ID=$($HTTP get "$INSTANCE_HOST/instances-name-to-id/$GROUP/$SOURCE_INSTANCE" "Authorization: Bearer $ACCESS_TOKEN")

echo "{
  \"name\": \"$DESTINATION_INSTANCE\",
  \"groupName\": \"$GROUP\",
  \"stackName\": \"$STACK\",
  \"sourceInstance\": $SOURCE_INSTANCE_ID,
  \"requiredParameters\": [
    {
      \"name\": \"PGADMIN_USERNAME\",
      \"value\": \"$PGADMIN_USERNAME\"
    },
    {
      \"name\": \"PGADMIN_PASSWORD\",
      \"value\": \"$PGADMIN_PASSWORD\"
    }
  ],
  \"optionalParameters\": [
    {
      \"name\": \"INSTANCE_TTL\",
      \"value\": \"$INSTANCE_TTL\"
    }
  ]
}" | $HTTP post "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
