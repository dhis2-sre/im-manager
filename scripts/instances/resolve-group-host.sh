#!/usr/bin/env bash
# Resolves a group name to its base hostname via the IM API.
# Falls back to <group>.im.dhis2.org with a warning if the lookup fails.

resolve_group_host() {
  local group="$1"
  local fallback="$group.im.dhis2.org"
  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  : "${HTTP:=http}"

  if [ -z "${IM_HOST:-}" ]; then
    echo "WARNING: IM_HOST not set; using $fallback" >&2
    echo "$fallback"
    return
  fi

  local token
  if ! token=$(cd "$script_dir" && source ./auth.sh 2>/dev/null && printf '%s' "$ACCESS_TOKEN"); then
    echo "WARNING: IM auth failed; using $fallback" >&2
    echo "$fallback"
    return
  fi

  if [ -z "$token" ]; then
    echo "WARNING: IM auth returned no token; using $fallback" >&2
    echo "$fallback"
    return
  fi

  local host
  if ! host=$($HTTP --check-status get "$IM_HOST/groups/$group" "Authorization: Bearer $token" 2>/dev/null | jq -r '.hostname // empty'); then
    echo "WARNING: IM group lookup failed for '$group'; using $fallback" >&2
    echo "$fallback"
    return
  fi

  if [ -z "$host" ]; then
    echo "WARNING: IM returned no hostname for '$group'; using $fallback" >&2
    echo "$fallback"
    return
  fi

  echo "$host"
}
