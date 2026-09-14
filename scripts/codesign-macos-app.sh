#!/usr/bin/env bash
# Sign a macOS .app for distribution (nested binaries first).
#
# Release CI: set APPLE_SIGNING_IDENTITY to your "Developer ID Application: …" name.
# Local dev:  omit it (defaults to ad-hoc `-`).
set -euo pipefail

APP="${1:?usage: codesign-macos-app.sh <MyApp.app>}"

if [ "$(uname -s)" != "Darwin" ]; then
  echo "codesign-macos-app.sh is macOS-only" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IDENTITY="${APPLE_SIGNING_IDENTITY:--}"
ENTITLEMENTS="${APPLE_ENTITLEMENTS:-$ROOT/desktop/src-tauri/entitlements.plist}"

sign_one() {
  local target="$1"
  local args=(--force --sign "$IDENTITY")
  if [ "$IDENTITY" != "-" ]; then
    args+=(--options runtime --timestamp)
    if [ -f "$ENTITLEMENTS" ]; then
      args+=(--entitlements "$ENTITLEMENTS")
    fi
  fi
  codesign "${args[@]}" "$target"
}

# Deepest Mach-O / dylibs first, then the bundle root.
while IFS= read -r -d '' f; do
  if file "$f" | grep -qE 'Mach-O|dynamically linked shared library'; then
    sign_one "$f"
  fi
done < <(find "$APP/Contents" -type f \( -perm -111 -o -name '*.dylib' -o -name '*.so' \) -print0 2>/dev/null | sort -rz)

sign_one "$APP"
codesign --verify --verbose=2 "$APP"

if [ "$IDENTITY" = "-" ]; then
  echo "ad-hoc signed $APP (Gatekeeper will warn — use Developer ID + notarization for releases)"
else
  echo "signed $APP with $IDENTITY"
fi
