#!/usr/bin/env bash
set -euo pipefail

REQUIRED_COMMANDS=("tr" "head" "fold" "shuf" "sed" "chmod" "cp" "openssl" "awk")
MISSING_COMMANDS=()

for cmd in "${REQUIRED_COMMANDS[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    MISSING_COMMANDS+=("$cmd")
  fi
done

if [ ${#MISSING_COMMANDS[@]} -ne 0 ]; then
  echo "Error: The following required commands are not available:" >&2
  printf "  - %s\n" "${MISSING_COMMANDS[@]}" >&2
  echo "" >&2
  echo "Please install the missing commands and try again." >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_FILE="$PROJECT_ROOT/.env"
TEMPLATE_FILE="$PROJECT_ROOT/.env.example"

if [ -f "$OUTPUT_FILE" ]; then
  echo "Error: A '$OUTPUT_FILE' file already exists." >&2
  exit 1
fi

if [ ! -f "$TEMPLATE_FILE" ]; then
  echo "Error: Template '$TEMPLATE_FILE' not found!" >&2
  exit 1
fi

LENGTH=32
CHARSET='A-Za-z0-9_=.-'

generate_password() {
  local password=""
  password+=$(LC_ALL=C tr -dc '[:upper:]' < /dev/urandom | head -c 1)
  password+=$(LC_ALL=C tr -dc '[:lower:]' < /dev/urandom | head -c 1)
  password+=$(LC_ALL=C tr -dc '0-9' < /dev/urandom | head -c 1)
  password+=$(LC_ALL=C tr -dc '_=.-' < /dev/urandom | head -c 1)
  local remaining=$((LENGTH - 4))
  password+=$(LC_ALL=C tr -dc "$CHARSET" < /dev/urandom | head -c "$remaining")
  echo "$password" | fold -w1 | shuf | tr -d '\n'
}

DATABASE_PASSWORD=$(generate_password)
MINIO_ROOT_PASSWORD=$(generate_password)
MINIO_PASSWORD=$(generate_password)
INSTANCE_PARAMETER_ENCRYPTION_KEY=$(openssl rand -hex 16)
REFRESH_TOKEN_SECRET_KEY=$(generate_password)
SESSION_SECRET=$(generate_password)
ADMIN_USER_PASSWORD=$(generate_password)
E2E_TEST_USER_PASSWORD=$(generate_password)

# Detect GNU vs BSD sed
if sed --version >/dev/null 2>&1; then
  SED_FLAGS=(-i)
else
  SED_FLAGS=(-i '')
fi

cp "$TEMPLATE_FILE" "$OUTPUT_FILE"

update_env_var() {
  local key="$1"
  local value="$2"
  sed "${SED_FLAGS[@]}" "s|^${key}=.*|${key}=${value}|" "$OUTPUT_FILE"
}

update_env_var "DATABASE_PASSWORD" "$DATABASE_PASSWORD"
update_env_var "MINIO_ROOT_PASSWORD" "$MINIO_ROOT_PASSWORD"
update_env_var "MINIO_PASSWORD" "$MINIO_PASSWORD"
update_env_var "INSTANCE_PARAMETER_ENCRYPTION_KEY" "$INSTANCE_PARAMETER_ENCRYPTION_KEY"
update_env_var "REFRESH_TOKEN_SECRET_KEY" "$REFRESH_TOKEN_SECRET_KEY"
update_env_var "SESSION_SECRET" "$SESSION_SECRET"
update_env_var "ADMIN_USER_PASSWORD" "$ADMIN_USER_PASSWORD"
update_env_var "E2E_TEST_USER_PASSWORD" "$E2E_TEST_USER_PASSWORD"

# Generate RSA private key with newlines inlined as \n
PRIVATE_KEY=$(openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 2>/dev/null | awk '{printf "%s\\n", $0}' | sed 's/\\n$//')

# Populate PRIVATE_KEY — use ENVIRON so awk doesn't interpret \n as a newline
PRIVATE_KEY="$PRIVATE_KEY" awk '/^PRIVATE_KEY=""/ { print "PRIVATE_KEY=\"" ENVIRON["PRIVATE_KEY"] "\""; next } { print }' "$OUTPUT_FILE" \
  > "${OUTPUT_FILE}.tmp" && mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"

chmod u+rw,go-rwx "$OUTPUT_FILE"

# Set ADMIN_USER_EMAIL if provided
if [ -n "${GEN_ADMIN_USER_EMAIL:-}" ]; then
  update_env_var "ADMIN_USER_EMAIL" "$GEN_ADMIN_USER_EMAIL"
fi

# Configure SOPS for local dev if age-keygen is available
if command -v age-keygen >/dev/null 2>&1; then
  SOPS_AGE_KEY=$(age-keygen 2>/dev/null | grep '^AGE-SECRET-KEY-')
  sed "${SED_FLAGS[@]}" "s|^#SOPS_AGE_KEY=|SOPS_AGE_KEY=${SOPS_AGE_KEY}|" "$OUTPUT_FILE"
  sed "${SED_FLAGS[@]}" "s|^SOPS_KMS_ARN=arn:aws:kms:eu-central-1:767224633206:alias/im-nonprod-secrets|#SOPS_KMS_ARN=arn:aws:kms:eu-central-1:767224633206:alias/im-nonprod-secrets|" "$OUTPUT_FILE"
fi

echo "A new .env has been generated at $OUTPUT_FILE"
echo ""
echo "Generated:"
echo "  - DATABASE_PASSWORD"
echo "  - MINIO_ROOT_PASSWORD, MINIO_PASSWORD"
echo "  - INSTANCE_PARAMETER_ENCRYPTION_KEY"
echo "  - REFRESH_TOKEN_SECRET_KEY, SESSION_SECRET"
echo "  - ADMIN_USER_PASSWORD, E2E_TEST_USER_PASSWORD"
echo "  - PRIVATE_KEY (RSA 2048-bit)"
if command -v age-keygen >/dev/null 2>&1; then
  echo "  - SOPS_AGE_KEY (age key; SOPS_KMS_ARN commented out)"
fi
echo ""
echo "Still requires manual configuration:"
echo "  - SMTP_USERNAME, SMTP_PASSWORD"
echo "  - AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY"
echo "  - DOCKER_HUB_USERNAME, DOCKER_HUB_PASSWORD"
echo "  - GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET (if using Google SSO)"
if ! command -v age-keygen >/dev/null 2>&1; then
  echo "  - SOPS_AGE_KEY or SOPS_KMS_ARN (age-keygen not found)"
fi
