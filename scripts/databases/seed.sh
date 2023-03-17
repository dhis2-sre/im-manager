#!/usr/bin/env bash

set -euo pipefail

GROUP=$1

function createDatabase() {
  echo "Downloading database... $1"
  curl -C - "$2" -o "$HOME/Downloads/$1.sql.gz"
  #curl "$2" -o "$HOME/Downloads/$1.sql.gz"

  echo "Login..."
  source ./auth.sh

  echo "Uploading database...$1"
  ./upload.sh "$GROUP" "$HOME/Downloads/$1.sql.gz"
}

## https://databases.dhis2.org/
createDatabase "Sierra Leone - 2.39.0" https://databases.dhis2.org/sierra-leone/2.39.0/dhis2-db-sierra-leone.sql.gz
createDatabase "Sierra Leone - 2.38.1" https://databases.dhis2.org/sierra-leone/2.38.1/dhis2-db-sierra-leone.sql.gz
createDatabase "Sierra Leone - 2.37.7" https://databases.dhis2.org/sierra-leone/2.37.6/dhis2-db-sierra-leone.sql.gz
createDatabase "Sierra Leone - 2.36.11" https://databases.dhis2.org/sierra-leone/2.36.0/dhis2-db-sierra-leone.sql.gz

./list.sh | jq '.[].Databases[]? | .ID, .Name, .Url'
