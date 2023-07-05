#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

DATABASES=$*

echo "Database(s): $DATABASES"

delete() {
  $HTTP delete "$IM_HOST/databases/$1" "Authorization: Bearer $ACCESS_TOKEN"
}

for DATABASE in $DATABASES; do
  delete "$DATABASE" &
done

# shellcheck disable=SC2046
wait $(jobs -p)
