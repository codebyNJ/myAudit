#!/usr/bin/env bash
# Notarize and staple a DMG (requires Apple Developer credentials in env).
set -euo pipefail

DMG="${1:?usage: notarize-dmg.sh <file.dmg>}"
PROFILE="${NOTARY_KEYCHAIN_PROFILE:-myaudit-notarize}"

SIGNED_RELEASE=false
if [ -n "${APPLE_SIGNING_IDENTITY:-}" ] && [ "${APPLE_SIGNING_IDENTITY}" != "-" ]; then
  SIGNED_RELEASE=true
fi

if [ -z "${APPLE_ID:-}" ] || [ -z "${APPLE_APP_SPECIFIC_PASSWORD:-}" ] || [ -z "${APPLE_TEAM_ID:-}" ]; then
  if [ "$SIGNED_RELEASE" = true ]; then
    echo "::error::Developer ID signing is configured but APPLE_ID / APPLE_APP_SPECIFIC_PASSWORD / APPLE_TEAM_ID are missing" >&2
    exit 1
  fi
  echo "skip notarization (set APPLE_ID, APPLE_APP_SPECIFIC_PASSWORD, APPLE_TEAM_ID)" >&2
  exit 0
fi

echo "==> storing notary credentials ($PROFILE)"
xcrun notarytool store-credentials "$PROFILE" \
  --apple-id "$APPLE_ID" \
  --team-id "$APPLE_TEAM_ID" \
  --password "$APPLE_APP_SPECIFIC_PASSWORD"

xattr -cr "$DMG" 2>/dev/null || true

echo "==> submitting $DMG to Apple notary service"
xcrun notarytool submit "$DMG" --keychain-profile "$PROFILE" --wait --timeout 20m

echo "==> stapling notarization ticket"
xcrun stapler staple "$DMG"
xcrun stapler validate "$DMG"
echo "notarized and stapled: $DMG"
