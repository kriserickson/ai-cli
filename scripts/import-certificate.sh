#!/bin/sh
# Import an Apple Developer ID certificate into a temporary keychain.
# Called during CI release builds so codesign can find the identity.
#
# Usage: import-certificate.sh
#
# Required environment variables:
#   CERTIFICATE_BASE64     - Base64-encoded .p12 certificate
#   CERTIFICATE_PASSWORD   - Password used when exporting the .p12
#
# Outputs:
#   KEYCHAIN_FILE          - Written to GITHUB_ENV for subsequent steps

set -e

if [ -z "${CERTIFICATE_BASE64:-}" ] || [ -z "${CERTIFICATE_PASSWORD:-}" ]; then
  echo "Certificate credentials not set, skipping import"
  exit 0
fi

CERT_FILE="$(mktemp /tmp/cert.XXXXXX.p12)"
KEYCHAIN_FILE="$(mktemp /tmp/keychain.XXXXXX.keychain-db)"
KEYCHAIN_PASSWORD="$(openssl rand -hex 16)"

echo "${CERTIFICATE_BASE64}" | openssl base64 -d -out "${CERT_FILE}"

echo "Creating temporary keychain..."
security create-keychain -p "${KEYCHAIN_PASSWORD}" "${KEYCHAIN_FILE}"
security set-keychain-settings -lut 21600 "${KEYCHAIN_FILE}"
security unlock-keychain -p "${KEYCHAIN_PASSWORD}" "${KEYCHAIN_FILE}"

echo "Importing certificate..."
security import "${CERT_FILE}" \
  -P "${CERTIFICATE_PASSWORD}" \
  -A -t cert -f pkcs12 \
  -k "${KEYCHAIN_FILE}"
security set-key-partition-list \
  -S apple-tool:,apple:,codesign: \
  -k "${KEYCHAIN_PASSWORD}" \
  "${KEYCHAIN_FILE}"
security list-keychains -d user -s \
  "${KEYCHAIN_FILE}" \
  $(security list-keychains -d user | tr -d '"')

rm -f "${CERT_FILE}"

# Export keychain path for subsequent workflow steps
if [ -n "${GITHUB_ENV:-}" ]; then
  echo "KEYCHAIN_FILE=${KEYCHAIN_FILE}" >> "${GITHUB_ENV}"
fi

echo "Certificate imported into keychain: ${KEYCHAIN_FILE}"
