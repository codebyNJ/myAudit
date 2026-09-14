#!/usr/bin/env bash
# Build and open a local DMG for visual review (does not push or release).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="${1:-$ROOT/desktop/src-tauri/target/aarch64-apple-darwin/release/bundle/macos/myAudit.app}"
OUT="${2:-/tmp/myaudit-dmg-preview}"

if [ ! -d "$APP" ]; then
  echo "App bundle not found: $APP" >&2
  echo "Build first: cd desktop && npx tauri build --target aarch64-apple-darwin --bundles app" >&2
  exit 1
fi

python3 -c "import PIL" 2>/dev/null || pip3 install -q pillow

echo "==> validate layout + generate background"
python3 "$ROOT/scripts/gen-dmg-background.py"

echo "==> debug overlay"
python3 "$ROOT/scripts/dmg-layout-preview.py"

chmod +x "$ROOT/scripts/repack-dmg.sh" "$ROOT/scripts/codesign-macos-app.sh"
"$ROOT/scripts/codesign-macos-app.sh" "$APP" 2>/dev/null || true

echo "==> pack DMG (create-dmg, electron-builder layout)"
"$ROOT/scripts/repack-dmg.sh" preview "$APP" "$OUT"

open "$OUT/myAudit-preview-macOS.dmg"
echo "debug overlay: $ROOT/desktop/dmg-layout-debug.png"
