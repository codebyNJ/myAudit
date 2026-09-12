#!/usr/bin/env bash
# Install myAudit desktop app from the latest GitHub Release.
# Usage: curl -fsSL https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.sh | bash
set -euo pipefail

REPO="${MYAUDIT_INSTALL_REPO:-codebyNJ/myAudit}"
VERSION="${MYAUDIT_VERSION:-latest}"
API="https://api.github.com/repos/${REPO}/releases/${VERSION}"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin)
    case "$ARCH" in
      arm64) PATTERN='myAudit-.*-macOS-AppleSilicon\.dmg$' ;;
      x86_64) PATTERN='myAudit-.*-macOS-Intel\.dmg$' ;;
      *) echo "unsupported macOS arch: $ARCH" >&2; exit 1 ;;
    esac
    ;;
  Linux)
    PATTERN='myAudit-.*-Linux-x86_64\.AppImage$'
    ;;
  *)
    echo "use scripts/install.ps1 on Windows" >&2
    exit 1
    ;;
esac

echo "==> fetching release metadata from ${REPO}"
JSON="$(curl -fsSL -H "Accept: application/vnd.github+json" "$API")"

URL="$(echo "$JSON" | python3 -c 'import json,re,sys; d=json.load(sys.stdin); p=re.compile(sys.argv[1]); print(next((a["browser_download_url"] for a in d.get("assets",[]) if p.search(a.get("name",""))), ""))' "$PATTERN")"

if [ -z "$URL" ]; then
  echo "no matching installer found" >&2
  echo "see https://github.com/${REPO}/releases" >&2
  exit 1
fi

FILE="$(basename "$URL")"
TMP="${TMPDIR:-/tmp}/myaudit-install"
mkdir -p "$TMP"
DEST="$TMP/$FILE"

echo "==> downloading $FILE"
curl -fsSL -o "$DEST" "$URL"

case "$OS" in
  Darwin)
    echo "==> open the disk image and drag myAudit to Applications"
    open "$DEST"
    ;;
  Linux)
    chmod +x "$DEST"
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    TARGET="$INSTALL_DIR/myAudit"
    cp -f "$DEST" "$TARGET"
    chmod +x "$TARGET"
    echo "==> installed to $TARGET"
    echo "    add $INSTALL_DIR to PATH if needed, then run: myAudit"
    ;;
esac
