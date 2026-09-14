#!/usr/bin/env bash
# Rename Tauri bundle outputs to predictable download names for GitHub Releases.
set -euo pipefail

VERSION="${1:?usage: package-release.sh <version> <bundle-dir> <triple> <out-dir>}"
BUNDLE_DIR="${2:?}"
TRIPLE="${3:?}"
OUT="${4:-dist}"

mkdir -p "$OUT"
V="$VERSION"

copy_one() {
  local src="$1" dest="$2"
  if [ -f "$src" ]; then
    cp "$src" "$OUT/$dest"
    echo "  $dest"
  fi
}

case "$TRIPLE" in
  x86_64-pc-windows-msvc)
    WIN_EXE="$(find "$BUNDLE_DIR/nsis" -maxdepth 1 -name '*-setup.exe' 2>/dev/null | head -1)"
    [ -z "$WIN_EXE" ] && WIN_EXE="$(find "$BUNDLE_DIR/nsis" -maxdepth 1 -name '*.exe' 2>/dev/null | head -1)"
    copy_one "$WIN_EXE" "myAudit-${V}-Windows-x86_64-setup.exe"
    copy_one "$(find "$BUNDLE_DIR/msi" -maxdepth 1 -name '*.msi' 2>/dev/null | head -1)" "myAudit-${V}-Windows-x86_64.msi"
    ;;
  *)
    echo "unknown triple: $TRIPLE" >&2
    exit 1
    ;;
esac

if [ -z "$(ls -A "$OUT" 2>/dev/null)" ]; then
  echo "no release artifacts found under $BUNDLE_DIR" >&2
  exit 1
fi

ls -la "$OUT"
