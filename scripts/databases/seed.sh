#!/usr/bin/env bash

set -euo pipefail

GROUP=$1
shift
VERSIONS=$*

DHIS2_RELEASES_URL="https://releases.dhis2.org/v1/versions/stable.json"
DB_DUMP_FORMAT='.sql.gz'

function createDatabase() {
  local db_name=${1}${DB_DUMP_FORMAT}

  echo "Downloading database $db_name ..."

  mkdir -p "$HOME/Downloads"
  curl -C - "$2" -o "$HOME/Downloads/$db_name"

  echo "Login ..."
  rm .access_token_cache # to make sure we're not using an expired token if we're seeding a lot of databases
  source ./auth.sh

  echo "Uploading database $db_name ..."
  ./upload.sh "$GROUP" "sierra-leone/$db_name" "$HOME/Downloads/$db_name"
  echo # empty line to improve output readability
}

if [[ -z "$*" ]]; then
  VERSIONS=("dev")

  # Filter out all supported versions, including patch versions (like 2.40, 2.40.0, 2.39, 2.39.0, etc)
  # shellcheck disable=SC2207
  VERSIONS+=($(
    curl -fsSL "$DHIS2_RELEASES_URL" | jq -r "[.versions[] | select(.supported == true) |
      [.name] + [(.patchVersions[] | select(.hotfix != true) | .name)]] | flatten | .[]"
  ))
fi

# shellcheck disable=SC2145
echo "Seeding the following database versions: ${VERSIONS[@]}"

for VERSION in "${VERSIONS[@]}"; do
  createDatabase "$VERSION" "https://databases.dhis2.org/sierra-leone/$VERSION/dhis2-db-sierra-leone.sql.gz"
done
