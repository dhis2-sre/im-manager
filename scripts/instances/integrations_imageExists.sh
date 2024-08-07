#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

repository=$1
tag=$2

$HTTP get "$IM_HOST/integrations/image-exists/$repository/$tag" "Authorization: Bearer $ACCESS_TOKEN"
