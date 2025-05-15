#!/bin/bash

set -euo pipefail

# Function to display usage
print_usage() {
  echo "Usage: $0 -g GROUP -i INSTANCES -n NAME (-f FILE | -a APP_ID) [-s START_NUM] [-u USERNAME] [-p PASSWORD]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i INSTANCES  Number of instances"
  echo "  -n NAME       Instance name prefix"
  echo "  -f FILE       File to upload"
  echo "  -a APP_ID     App Hub ID to install"
  echo "  -s START_NUM  Starting instance number (default: 1)"
  echo "  -u USERNAME   Username for authentication (default: admin)"
  echo "  -p PASSWORD   Password for authentication (default: district)"
  exit 1
}

# Default values
START_NUM=1
USERNAME="admin"
PASSWORD="district"

# Parse command line arguments
while getopts ":g:i:n:s:u:p:f:a:h" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) INSTANCES="$OPTARG" ;;
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

# Verify required arguments
if [ -z "${GROUP:-}" ] || [ -z "${INSTANCES:-}" ] || [ -z "${NAME:-}" ]; then
  echo "Error: Missing required arguments" >&2
  print_usage
fi

# Verify either FILE or APP_ID is provided, but not both
if [ -z "${FILE:-}" ] && [ -z "${APP_ID:-}" ]; then
  echo "Error: Either -f FILE or -a APP_ID must be provided" >&2
  print_usage
elif [ -n "${FILE:-}" ] && [ -n "${APP_ID:-}" ]; then
  echo "Error: Cannot specify both -f FILE and -a APP_ID" >&2
  print_usage
fi

# If FILE is provided, verify it exists
if [ -n "${FILE:-}" ] && [ ! -f "$FILE" ]; then
  echo "Error: File '$FILE' not found" >&2
  exit 1
fi

for ((i = START_NUM; i < INSTANCES + 1; i++)); do
  # zero pad the number
  zi=$(printf "%02d" $i)
  if [ -n "${FILE:-}" ]; then
    # File upload mode
    http --auth "$USERNAME:$PASSWORD" \
      --form POST "https://$GROUP.im.dhis2.org/$NAME-$zi/api/apps" \
      file@"$FILE"
  else
    echo "Installing app $APP_ID on $NAME-$zi"
    echo "http --auth $USERNAME:$PASSWORD POST https://$GROUP.im.dhis2.org/$NAME-$zi/api/appHub/$APP_ID"
    # App Hub installation mode
    http --auth "$USERNAME:$PASSWORD" \
      POST "https://$GROUP.im.dhis2.org/$NAME-$zi/api/appHub/$APP_ID"
  fi
done 