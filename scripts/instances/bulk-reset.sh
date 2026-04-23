#!/bin/bash

set -euo pipefail

# Function to display usage
print_usage() {
  echo "Usage: $0 -g GROUP -i INSTANCES -n NAME [-s START_NUM] [-f]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i INSTANCES  Number of instances"
  echo "  -n NAME       Instance name prefix"
  echo "  -s START_NUM  Starting instance number (default: 1)"
  echo "  -f           Force reset (only reset if 503 error)"
  exit 1
}

# Default values
START_NUM=1
FORCE_RESET="false"

# Parse command line arguments
while getopts ":g:i:n:s:fh" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) INSTANCES="$OPTARG" ;;
    n) NAME="$OPTARG" ;;
    s) START_NUM="$OPTARG" ;;
    f) FORCE_RESET="true" ;;
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


# Main loop
for ((i = START_NUM; i < INSTANCES + 1; i++)); do
  # Ensure each deploy get a fresh access token
  rm -f .access_token_cache
  source ./auth.sh
  # zero pad the number
  zi=$(printf "%02d" $i)
  instance_name="$NAME-$zi"
  
  # Get instance details and extract core instance ID
  instance_data=$(./findByName.sh "$GROUP" "$instance_name")


  instance_id=$(echo "$instance_data" | jq -r '.instances[] | select(.stackName=="dhis2-core") | .id')
  
  if [ -z "$instance_id" ]; then
    echo "Error: Could not find core instance ID for $instance_name" >&2
    continue
  fi

  # If an "f" argument (force reset) is given, then check https://qa.im.dhis2.org/$instance_name
  # and if it returns a 503 - only then run the reset.
  # if no "f" argument then always run the reset
  if [ "$FORCE_RESET" = "true" ]; then
    if curl -s -o /dev/null -w "%{http_code}" https://qa.im.dhis2.org/$instance_name | grep -q "503"; then
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
  
