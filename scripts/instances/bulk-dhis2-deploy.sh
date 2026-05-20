#!/bin/bash
# Deploys N DHIS2 instances in a group by invoking deploy-dhis2.sh per instance.
# Refreshes the IM access token before each deploy.

set -euo pipefail

print_usage() {
  echo "Usage: $0 -g GROUP -i END_NUM -n NAME [-s START_NUM] [-u USERNAME] [-p PASSWORD]"
  echo "Options:"
  echo "  -g GROUP      DHIS2 group/domain"
  echo "  -i END_NUM    Last instance number (range is [START_NUM, END_NUM])"
  echo "  -n NAME       Instance name prefix"
  echo "  -s START_NUM  Starting instance number (default: 1)"
  exit 1
}

START_NUM=1

while getopts ":g:i:n:s:h" opt; do
  case $opt in
    g) GROUP="$OPTARG" ;;
    i) END_NUM="$OPTARG" ;;
    n) NAME="$OPTARG" ;;
    s) START_NUM="$OPTARG" ;;
    h) print_usage ;;
    \?) echo "Invalid option -$OPTARG" >&2; print_usage ;;
    :) echo "Option -$OPTARG requires an argument" >&2; print_usage ;;
  esac
done

if [ -z "${GROUP:-}" ] || [ -z "${END_NUM:-}" ] || [ -z "${NAME:-}" ]; then
  echo "Error: Missing required arguments" >&2
  print_usage
fi

export STARTUP_PROBE_PERIOD_SECONDS=10

export DATABASE_ID=test-dbs-sierra-leone-dev-sql-gz
export DATABASE_SIZE=20Gi

export IMAGE_REPOSITORY=core-dev
export IMAGE_PULL_POLICY=Always
export IMAGE_TAG=latest
export INSTANCE_TTL=432000 # 5 days

export CORE_RESOURCES_REQUESTS_CPU=500m # 250m
export CORE_RESOURCES_REQUESTS_MEMORY=2500Mi # 1500Mi
#export ALLOW_SUSPEND=false


for ((i = START_NUM; i < END_NUM + 1; i++)); do
  # Ensure each deploy get a fresh access token
  rm -f .access_token_cache
  source ./auth.sh
  zi=$(printf "%02d" $i)
  ./deploy-dhis2.sh "$GROUP" "$NAME-$zi"
done
