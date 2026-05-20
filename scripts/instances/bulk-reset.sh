#!/bin/bash
# Resets the core database on a range of DHIS2 instances via reset.sh.
# With -b, only resets instances returning HTTP 503 (broken).

set -euo pipefail

print_usage() {
  echo "Usage: $0 -g GROUP -i END_NUM -n NAME [-s START_NUM] [-b]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i END_NUM    Last instance number (range is [START_NUM, END_NUM])"
  echo "  -n NAME       Instance name prefix"
  echo "  -s START_NUM  Starting instance number (default: 1)"
  echo "  -b           Only reset instances returning HTTP 503 (broken)"
  exit 1
}

START_NUM=1
BROKEN_ONLY="false"

while getopts ":g:i:n:s:bh" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) END_NUM="$OPTARG" ;;
    n) NAME="$OPTARG" ;;
    s) START_NUM="$OPTARG" ;;
    b) BROKEN_ONLY="true" ;;
    h) print_usage ;;
    \?) echo "Invalid option -$OPTARG" >&2; print_usage ;;
    :) echo "Option -$OPTARG requires an argument" >&2; print_usage ;;
  esac
done

if [ -z "${GROUP:-}" ] || [ -z "${END_NUM:-}" ] || [ -z "${NAME:-}" ]; then
  echo "Error: Missing required arguments" >&2
  print_usage
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/resolve-group-host.sh"
GROUP_HOST=$(resolve_group_host "$GROUP")

for ((i = START_NUM; i < END_NUM + 1; i++)); do
  # Ensure each deploy get a fresh access token
  rm -f .access_token_cache
  source ./auth.sh
  zi=$(printf "%02d" $i)
  instance_name="$NAME-$zi"

  instance_data=$(./findByName.sh "$GROUP" "$instance_name")

  instance_id=$(echo "$instance_data" | jq -r '.instances[] | select(.stackName=="dhis2-core") | .id')

  if [ -z "$instance_id" ]; then
    echo "Error: Could not find core instance ID for $instance_name" >&2
    continue
  fi

  if [ "$BROKEN_ONLY" = "true" ]; then
    if curl -s -o /dev/null -w "%{http_code}" "https://$GROUP_HOST/$instance_name" | grep -q "503"; then
      echo "503 returned, running reset"
      ./reset.sh "$instance_id"
    else
      echo "503 not returned, skipping reset"
    fi
  else
    echo "./reset.sh $instance_id"
    ./reset.sh "$instance_id"
  fi
done
