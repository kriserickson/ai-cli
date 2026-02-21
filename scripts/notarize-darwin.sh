#!/bin/sh
# Notarize signed macOS binaries after GoReleaser build.
# Finds darwin binaries in GoReleaser's dist/ directory and submits them
# to Apple's notary service.
#
# Usage: notarize-darwin.sh
#
# Required environment variables:
#   APPLE_ID           - Apple ID email
#   APPLE_ID_PASSWORD  - App-specific password
#   APPLE_TEAM_ID      - Apple Developer Team ID

set -e

if [ -z "${APPLE_ID:-}" ] || [ -z "${APPLE_ID_PASSWORD:-}" ] || [ -z "${APPLE_TEAM_ID:-}" ]; then
  echo "Apple notarization credentials not set, skipping notarization"
  exit 0
fi

FOUND=0

for bin in dist/ai_darwin_*/ai; do
  [ -f "$bin" ] || continue
  FOUND=1

  echo "Notarizing: $bin"
  ZIP="$(mktemp /tmp/notarize.XXXXXX.zip)"
  ditto -c -k "$bin" "$ZIP"

  xcrun notarytool submit "$ZIP" \
    --apple-id "${APPLE_ID}" \
    --password "${APPLE_ID_PASSWORD}" \
    --team-id "${APPLE_TEAM_ID}" \
    --wait

  rm -f "$ZIP"
  echo "Notarized: $bin"
done

if [ "$FOUND" -eq 0 ]; then
  echo "No darwin binaries found in dist/, skipping notarization"
fi
