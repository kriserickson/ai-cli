#!/bin/sh
# Sign macOS binaries for distribution.
# Called by GoReleaser as a post-build hook.
#
# Usage: sign-darwin.sh <os> <binary-path>

set -e

OS="$1"
BINARY="$2"

if [ "$OS" != "darwin" ]; then
  exit 0
fi

if [ -z "${APPLE_SIGNING_IDENTITY:-}" ]; then
  echo "APPLE_SIGNING_IDENTITY not set, skipping code signing"
  exit 0
fi

echo "Signing: $BINARY"
codesign --force --options runtime \
  --sign "${APPLE_SIGNING_IDENTITY}" \
  --timestamp \
  "$BINARY"

echo "Verifying signature..."
codesign --verify --verbose "$BINARY"
echo "Signed and verified: $BINARY"
