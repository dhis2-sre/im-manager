#!/bin/sh

set -o pipefail

# The marker lives outside files/ so it is not swept into the next backup.
marker="$DHIS2_HOME/.im-filestore-seeded"

if [ -f "$marker" ]; then
  echo "Filestore already seeded, skipping..."
  exit 0
fi

if [ -z "${FILESTORE_DOWNLOAD_URL:-}" ]; then
  echo "No filestore to seed"
  exit 0
fi

apk add --no-cache curl tar gzip

echo "Filestore download URL: $FILESTORE_DOWNLOAD_URL"
echo "Seeding filesystem..."

tmp_file=$(mktemp)
trap 'rm -f "$tmp_file"' EXIT

if ! curl --connect-timeout 10 --retry 5 --retry-delay 1 --fail --show-error -L "$FILESTORE_DOWNLOAD_URL" > "$tmp_file"; then
  echo "Failed to download filestore from $FILESTORE_DOWNLOAD_URL"
  exit 1
fi

mkdir -p "$DHIS2_HOME/files"
if ! gunzip -c "$tmp_file" | tar xf - -C "$DHIS2_HOME/files"; then
  echo "Failed to extract filestore archive"
  exit 1
fi

# Write the marker only after a successful extract so a failed attempt retries.
echo "Seeded from $FILESTORE_DOWNLOAD_URL" > "$marker"

echo "Done seeding filesystem!"
