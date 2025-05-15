#!/bin/bash

set -euo pipefail

# Function to display usage
print_usage() {
  echo "Usage: $0 -g GROUP -i INSTANCES -n NAME [-s START_NUM] [-u USERNAME] [-p PASSWORD] [-c]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i INSTANCES  Number of instances"
  echo "  -n NAME       Instance name prefix"
  echo "  -s START_NUM  Starting instance number (default: 1)"
  echo "  -u USERNAME   Username for authentication (default: admin)"
  echo "  -p PASSWORD   Password for authentication (default: district)"
  echo "  -c           Conditional mode (only run if analytics returns 409)"
  exit 1
}

# Default values
START_NUM=1
USERNAME="admin"
PASSWORD="district"
CONDITIONAL="false"

# Parse command line arguments
while getopts ":g:i:n:s:u:p:ch" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) INSTANCES="$OPTARG" ;;
    n) NAME="$OPTARG" ;;
    s) START_NUM="$OPTARG" ;;
    u) USERNAME="$OPTARG" ;;
    p) PASSWORD="$OPTARG" ;;
    c) CONDITIONAL="true" ;;
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

for ((i = START_NUM; i < INSTANCES + 1; i++)); do
  # zero pad the number
  zi=$(printf "%02d" $i)
  echo "Processing $NAME-$zi"
  if [ "$CONDITIONAL" = "true" ]; then
    echo "Conditional mode"
    # Check analytics endpoint first
    status_code=$(curl -s -o /dev/null -w "%{http_code}" \
      --user "$USERNAME:$PASSWORD" \
      "https://$GROUP.im.dhis2.org/$NAME-$zi/api/analytics/dataValueSet.json?dimension=ou%3ATEQlaapDQoK%3BVth0fbpFcsO%3BbL4ooGhyHRQ%3BjmIPBj66vD6%3BqhqAxPSTUXp%3BLEVEL-wjP19dkFeIk&dimension=pe%3ALAST_12_MONTHS&dimension=dx%3AsB79w2hiLp8&showHierarchy=false&hierarchyMeta=false&includeMetadataDetails=true&includeNumDen=true&skipRounding=false&completedOnly=false")
    
    if [ "$status_code" = "409" ]; then
      echo "$NAME-$zi : Analytics returned 409, running"
      curl -s -X POST --user "$USERNAME:$PASSWORD" \
        "https://$GROUP.im.dhis2.org/$NAME-$zi/api/resourceTables/analytics"
    else
      echo "$NAME-$zi : Analytics returned $status_code, skipping"
    fi
  else
    # Always run analytics in non-conditional mode
    curl -s -X POST --user "$USERNAME:$PASSWORD" \
      "https://$GROUP.im.dhis2.org/$NAME-$zi/api/resourceTables/analytics"
  fi
done
