mc alias set myminio "http://{{ requiredEnv "INSTANCE_NAME" }}-minio:9000" dhisdhis dhisdhis

seed_file=myminio/dhis2/seeded.txt
if mc stat $seed_file >/dev/null 2>&1; then
  echo "Already seeded, skipping..."
else
  # Wait for MinIO to be ready, accounting for restart with consecutive checks
  timeout=120
  elapsed=0
  success_count=0
  required_successes=3

  while [ "$success_count" -lt "$required_successes" ]; do
    if curl --silent --fail "http://{{ requiredEnv "INSTANCE_NAME" }}-minio:9000/minio/health/ready"; then
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

  echo "MinIO is stable and ready!!!"

  DATABASE_URL="$HOSTNAME/databases/$DATABASE_ID"
  echo "DATABASE_URL: $DATABASE_URL"
  FILESTORE_ID=$(curl --connect-timeout 10 --retry 5 --retry-delay 1 --fail -L $DATABASE_URL --cookie "accessToken=$IM_ACCESS_TOKEN" | jq -r '.filestoreId')
  if [[ "$FILESTORE_ID" == "0" ]]; then
    echo "No filestore id associated with database"
  else
    echo "Filestore ID: $FILESTORE_ID"
    echo "Seeding..."

    tmp_file=$(mktemp)
    trap 'rm -f "$tmp_file"' EXIT  # Ensures cleanup on script exit
    FILESTORE_DOWNLOAD_URL="$HOSTNAME/databases/$FILESTORE_ID/download"
    curl --connect-timeout 10 --retry 5 --retry-delay 1 --fail -L "$FILESTORE_DOWNLOAD_URL" --cookie "accessToken=$IM_ACCESS_TOKEN" > "$tmp_file"

    tmp_dir=$(mktemp -d /tmp/minio.XXXXXX)
    trap 'rm -rf "$tmp_dir"' EXIT  # Ensures cleanup on script exit
    gunzip -c "$tmp_file" | tar xf - -C "$tmp_dir"
    chmod -R u+rwx,go+rx "$tmp_dir"

    mc cp --recursive "$tmp_dir"/* myminio/dhis2

    echo "Seeded from $FILESTORE_DOWNLOAD_URL" | mc pipe $seed_file

    # Clean up
    rm -f "$tmp_file"
    rm -rf "$tmp_dir"

    echo "Done seeding!"
  fi
fi

# Wait forever
tail -f /dev/null