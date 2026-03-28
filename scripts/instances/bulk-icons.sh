#!/bin/bash

set -euo pipefail

# Function to display usage
print_usage() {
  echo "Usage: $0 -g GROUP -i INSTANCES -n NAME [-s START_NUM] [-u USERNAME] [-p PASSWORD]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i INSTANCES  Number of instances"
  echo "  -n NAME       Instance name prefix"
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
while getopts ":g:i:n:s:u:p:h" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) INSTANCES="$OPTARG" ;;
    n) NAME="$OPTARG" ;;
    s) START_NUM="$OPTARG" ;;
    u) USERNAME="$OPTARG" ;;
    p) PASSWORD="$OPTARG" ;;
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

if [ -z "$START_NUM" ]; then
  START_NUM=1
fi

for ((i = $START_NUM; i < INSTANCES + 1; i++)); do
  # zero pad the number
  zi=$(printf "%02d" $i)
  http --auth "$USERNAME:$PASSWORD" patch "https://$GROUP.im.dhis2.org/$NAME-$zi/api/icons"
done
