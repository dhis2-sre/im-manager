#!/usr/bin/env bash

set -o pipefail

# Use 127.0.0.1 to ensure we hit the main MinIO container in the same pod.
MINIO_URL="http://127.0.0.1:9000"

timeout=60
elapsed=0
success_count=0
required_successes=5

while [ "$success_count" -lt "$required_successes" ]; do
  if curl --silent --fail "$MINIO_URL/minio/health/ready"; then
    success_count=$((success_count + 1))
    echo "MinIO health check $success_count/$required_successes passed"
  else
    success_count=0
    echo "MinIO health check failed, resetting counter"
  fi
  sleep 2
  elapsed=$((elapsed + 2))
  if [ "$elapsed" -ge "$timeout" ]; then
    echo "Timeout reached: MinIO is not ready after $timeout seconds."
    exit 1
  fi
done

echo "MinIO is stable and ready!"
mc alias set local "$MINIO_URL" dhisdhis dhisdhis

seed_file=local/dhis2/seeded.txt
if mc stat $seed_file >/dev/null 2>&1; then
  echo "Already seeded, skipping..."
else
  if [[ -z "${FILESTORE_DOWNLOAD_URL:-}" ]]; then
    echo "No filestore to seed"
  else
    echo "Filestore download URL: $FILESTORE_DOWNLOAD_URL"
    echo "Seeding..."

    tmp_file=$(mktemp)
    if ! curl --connect-timeout 10 --retry 5 --retry-delay 1 --fail --show-error -L "$FILESTORE_DOWNLOAD_URL" > "$tmp_file"; then
      echo "Failed to download filestore from $FILESTORE_DOWNLOAD_URL"
      exit 1
    fi

    tmp_dir=$(mktemp -d /tmp/minio.XXXXXX)
    trap 'rm -f "$tmp_file"; rm -rf "$tmp_dir"' EXIT
    gunzip -c "$tmp_file" | tar xf - -C "$tmp_dir"
    chmod -R u+rwx,go+rx "$tmp_dir"

    mc mirror "$tmp_dir"/ local/dhis2/

    echo "Seeded from $FILESTORE_DOWNLOAD_URL" | mc pipe $seed_file

    if ! mc stat $seed_file >/dev/null 2>&1; then
      echo "Seeding verification failed: $seed_file not found after upload"
      exit 1
    fi
    echo "Seeding verified: $seed_file exists"

    rm -f "$tmp_file"
    rm -rf "$tmp_dir"

    echo "Done seeding!"
  fi
fi

# Wait forever, if the sidecar container terminates Kubernetes will restart it
tail -f /dev/null
