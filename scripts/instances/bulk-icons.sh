#!/bin/bash
# Patches the icons collection across a range of DHIS2 instances.
# Uses HTTP Basic auth per instance (-u/-p, defaults admin/district).

set -euo pipefail

print_usage() {
  echo "Usage: $0 -g GROUP -i END_NUM -n NAME [-s START_NUM] [-u USERNAME] [-p PASSWORD]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i END_NUM    Last instance number (range is [START_NUM, END_NUM])"
  echo "  -n NAME       Instance name prefix"
  echo "  -s START_NUM  Starting instance number (default: 1)"
  echo "  -u USERNAME   Username for authentication (default: admin)"
  echo "  -p PASSWORD   Password for authentication (default: district)"
  exit 1
}

START_NUM=1
USERNAME="admin"
PASSWORD="district"

while getopts ":g:i:n:s:u:p:h" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) END_NUM="$OPTARG" ;;
    n) NAME="$OPTARG" ;;
    s) START_NUM="$OPTARG" ;;
    u) USERNAME="$OPTARG" ;;
    p) PASSWORD="$OPTARG" ;;
    h) print_usage ;;
    \?) echo "Invalid option -$OPTARG" >&2; print_usage ;;
    :) echo "Option -$OPTARG requires an argument" >&2; print_usage ;;
  esac
done

if [ -z "${GROUP:-}" ] || [ -z "${END_NUM:-}" ] || [ -z "${NAME:-}" ]; then
  echo "Error: Missing required arguments" >&2
  print_usage
fi

if [ -z "$START_NUM" ]; then
  START_NUM=1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/resolve-group-host.sh"
GROUP_HOST=$(resolve_group_host "$GROUP")

for ((i = $START_NUM; i < END_NUM + 1; i++)); do
  zi=$(printf "%02d" $i)
  http --auth "$USERNAME:$PASSWORD" patch "https://$GROUP_HOST/$NAME-$zi/api/icons"
done
