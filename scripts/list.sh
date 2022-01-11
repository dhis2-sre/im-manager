#!/usr/bin/env bash

#set -euxo pipefail

HTTP="http --verify=no --check-status"

$HTTP "$INSTANCE_HOST/instances" "Authorization: Bearer $ACCESS_TOKEN"
