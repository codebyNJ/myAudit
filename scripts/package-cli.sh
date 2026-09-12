#!/usr/bin/env bash
# Build a standalone myaudit-serve CLI archive for one target triple.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

VERSION="${1:?usage: package-cli.sh <version> <triple> <out-dir>}"
TRIPLE="${2:?}"
OUT="${3:-dist}"

echo "==> building web UI"
( cd web && npm ci --no-audit --no-fund && npm run build )
rm -rf internal/api/web/dist
cp -r web/dist internal/api/web/dist

EXT=""
case "$TRIPLE" in *windows*) EXT=".exe" ;; esac

case "$TRIPLE" in
  aarch64-apple-darwin)      export GOOS=darwin  GOARCH=arm64 ;;
  x86_64-apple-darwin)       export GOOS=darwin  GOARCH=amd64 ;;
  x86_64-unknown-linux-gnu)  export GOOS=linux   GOARCH=amd64 ;;
  aarch64-unknown-linux-gnu) export GOOS=linux   GOARCH=arm64 ;;
  x86_64-pc-windows-msvc)    export GOOS=windows GOARCH=amd64 ;;
  *) echo "unmapped triple: $TRIPLE" >&2; exit 1 ;;
esac

mkdir -p "$OUT"
BIN="myaudit-serve$EXT"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$OUT/$BIN" ./cmd/serve
chmod +x "$OUT/$BIN"

LABEL="$TRIPLE"
ARCHIVE="myaudit-serve-${VERSION}-${LABEL}.tar.gz"
case "$TRIPLE" in *windows*)
  ARCHIVE="myaudit-serve-${VERSION}-${LABEL}.zip"
  (cd "$OUT" && zip -q "$ARCHIVE" "$BIN" && rm "$BIN")
  ;;
*)
  (cd "$OUT" && tar -czf "$ARCHIVE" "$BIN" && rm "$BIN")
  ;;
esac

echo "==> $OUT/$ARCHIVE"
ls -la "$OUT"
