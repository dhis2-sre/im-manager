#!/usr/bin/env bash

set -euo pipefail

source ./auth.sh

GROUP=$1
NAME=$2
FILE=$3
shift 3
DESCRIPTION=$*

curl -X PUT --fail --progress-bar \
  -H "Authorization: $ACCESS_TOKEN" \
  -H "X-Upload-Group: $GROUP" \
  -H "X-Upload-Name: $NAME" \
  -H "X-Upload-Description: $DESCRIPTION" \
  -H "Content-Length: $(stat --printf="%s" "$FILE")" \
  --data-binary @"$FILE" \
  -L "$IM_HOST/databases" | cat
