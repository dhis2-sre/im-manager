#!/bin/bash
# Installs an app across a range of DHIS2 instances.
# Either uploads a local file (-f) or installs by App Hub ID (-a).

set -euo pipefail

print_usage() {
  echo "Usage: $0 -g GROUP -i END_NUM -n NAME (-f FILE | -a APP_ID) [-s START_NUM] [-u USERNAME] [-p PASSWORD]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i END_NUM    Last instance number (range is [START_NUM, END_NUM])"
  echo "  -n NAME       Instance name prefix"
  echo "  -f FILE       File to upload"
  echo "  -a APP_ID     App Hub ID to install"
  echo "  -s START_NUM  Starting instance number (default: 1)"
  echo "  -u USERNAME   Username for authentication (default: admin)"
  echo "  -p PASSWORD   Password for authentication (default: district)"
  exit 1
}

START_NUM=1
USERNAME="admin"
PASSWORD="district"

while getopts ":g:i:n:s:u:p:f:a:h" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) END_NUM="$OPTARG" ;;
    n) NAME="$OPTARG" ;;
    s) START_NUM="$OPTARG" ;;
    u) USERNAME="$OPTARG" ;;
    p) PASSWORD="$OPTARG" ;;
    f) FILE="$OPTARG" ;;
    a) APP_ID="$OPTARG" ;;
    h) print_usage ;;
    \?) echo "Invalid option -$OPTARG" >&2; print_usage ;;
    :) echo "Option -$OPTARG requires an argument" >&2; print_usage ;;
  esac
done

if [ -z "${GROUP:-}" ] || [ -z "${END_NUM:-}" ] || [ -z "${NAME:-}" ]; then
  echo "Error: Missing required arguments" >&2
  print_usage
fi

if [ -z "${FILE:-}" ] && [ -z "${APP_ID:-}" ]; then
  echo "Error: Either -f FILE or -a APP_ID must be provided" >&2
  print_usage
elif [ -n "${FILE:-}" ] && [ -n "${APP_ID:-}" ]; then
  echo "Error: Cannot specify both -f FILE and -a APP_ID" >&2
  print_usage
fi

if [ -n "${FILE:-}" ] && [ ! -f "$FILE" ]; then
  echo "Error: File '$FILE' not found" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/resolve-group-host.sh"
GROUP_HOST=$(resolve_group_host "$GROUP")

for ((i = START_NUM; i < END_NUM + 1; i++)); do
  zi=$(printf "%02d" $i)
  if [ -n "${FILE:-}" ]; then
    http --auth "$USERNAME:$PASSWORD" \
      --form POST "https://$GROUP_HOST/$NAME-$zi/api/apps" \
      file@"$FILE"
  else
    echo "Installing app $APP_ID on $NAME-$zi"
    http --auth "$USERNAME:$PASSWORD" \
      POST "https://$GROUP_HOST/$NAME-$zi/api/appHub/$APP_ID"
  fi
done