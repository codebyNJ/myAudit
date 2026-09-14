#!/usr/bin/env bash
# Build a styled drag-to-Applications DMG from a signed .app bundle.
#
# Uses create-dmg (same tool Tauri vendors as bundle_dmg.sh) with coordinates
# matching electron-builder's defaults: 540×380 window, icons at (130,220)
# and (410,220). Layout source: scripts/dmg-layout.json
set -euo pipefail

VERSION="${1:?usage: repack-dmg.sh <version> <MyApp.app> [out-dir]}"
APP="${2:?}"
OUT_DIR="${3:-dist}"

if [ "$(uname -s)" != "Darwin" ]; then
  echo "repack-dmg.sh is macOS-only" >&2
  exit 1
fi

if ! command -v create-dmg >/dev/null; then
  echo "create-dmg not found — install with: brew install create-dmg" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LAYOUT="$ROOT/scripts/dmg-layout.json"
BG="$ROOT/desktop/dmg-background.png"
VOLICON="$ROOT/desktop/src-tauri/icons/icon.icns"
VOLNAME="myAudit"

read -r WIN_W WIN_H WIN_X WIN_Y ICON_LX ICON_LY ICON_RX ICON_RY ICON_SIZE TEXT_SIZE < <(
  python3 -c "
import json
from pathlib import Path
L = json.loads(Path('$LAYOUT').read_text())
w, i = L['window'], L['icons']
print(w['width'], w['height'], w['pos'][0], w['pos'][1],
      i['left'][0], i['left'][1], i['right'][0], i['right'][1],
      i['size'], i['textSize'])
"
)

python3 "$ROOT/scripts/gen-dmg-background.py"
if [ ! -f "$BG" ]; then
  echo "missing DMG background: $BG" >&2
  exit 1
fi

# Verify @2x retina background (electron-builder / Tauri convention).
python3 -c "
import subprocess, sys
from PIL import Image
p = '$BG'
img = Image.open(p)
w, h = img.size
if (w, h) != (1080, 760):
    sys.exit(f'background must be 1080×760, got {w}×{h}')
dpi = subprocess.check_output(['sips','-g','dpiWidth',p], text=True)
if '144.000' not in dpi:
    sys.exit('background must be 144 DPI')
print(f'background OK: {w}×{h} @ 144dpi')
"

mkdir -p "$OUT_DIR"
STAGING="$(mktemp -d)"
trap 'rm -rf "$STAGING"' EXIT

cp -R "$APP" "$STAGING/$(basename "$APP")"

DMG="$OUT_DIR/myAudit-${VERSION}-macOS.dmg"
rm -f "$DMG"

ARGS=(
  --volname "$VOLNAME"
  --background "$BG"
  --window-pos "$WIN_X" "$WIN_Y"
  --window-size "$WIN_W" "$WIN_H"
  --icon-size "$ICON_SIZE"
  --text-size "$TEXT_SIZE"
  --icon "myAudit.app" "$ICON_LX" "$ICON_LY"
  --hide-extension "myAudit.app"
  --app-drop-link "$ICON_RX" "$ICON_RY"
  --format UDZO
  --no-internet-enable
  --overwrite
)
if [ -f "$VOLICON" ]; then
  ARGS+=(--volicon "$VOLICON")
fi

# create-dmg runs Finder AppleScript to set icon positions, background, and
# window bounds (same flow as Tauri's bundle_dmg.sh). No post-lock needed.
create-dmg "${ARGS[@]}" "$DMG" "$STAGING"

echo "  $(basename "$DMG") (${WIN_W}×${WIN_H}, icons ${ICON_SIZE}pt)"
ls -la "$OUT_DIR"
