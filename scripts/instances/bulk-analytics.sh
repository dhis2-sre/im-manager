#!/bin/bash

set -euo pipefail

GROUP=$1
INSTANCES=$2
NAME=$3

for ((i = 1; i < INSTANCES + 1; i++)); do
  http --auth "admin:district" post "https://$GROUP.im.dhis2.org/$NAME-$i/api/resourceTables/analytics"
done
