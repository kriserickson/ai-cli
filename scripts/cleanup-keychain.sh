#!/bin/sh
# Clean up the temporary keychain created by import-certificate.sh.
# Called as a post-release step with `if: always()` to ensure cleanup.
#
# Usage: cleanup-keychain.sh
#
# Expected environment variables:
#   KEYCHAIN_FILE - Path to the temporary keychain (set by import-certificate.sh)

set -e

if [ -z "${KEYCHAIN_FILE:-}" ]; then
  exit 0
fi

if [ ! -f "${KEYCHAIN_FILE}" ]; then
  exit 0
fi

echo "Removing temporary keychain: ${KEYCHAIN_FILE}"
security delete-keychain "${KEYCHAIN_FILE}" 2>/dev/null || true
echo "Keychain cleaned up"
