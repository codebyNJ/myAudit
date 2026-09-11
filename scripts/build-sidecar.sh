

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

TRIPLE="${TARGET_TRIPLE:-$(rustc -vV | awk '/^host:/ {print $2}')}"
[ -n "$TRIPLE" ] || { echo "could not determine target triple"; exit 1; }

echo "==> building web UI"
( cd web && npm ci --no-audit --no-fund && npm run build )

echo "==> embedding UI into the Go server"
rm -rf internal/api/web/dist
cp -r web/dist internal/api/web/dist

echo "==> building Go server for $TRIPLE"
EXT=""
case "$TRIPLE" in *windows*) EXT=".exe" ;; esac

case "$TRIPLE" in
  aarch64-apple-darwin)      export GOOS=darwin  GOARCH=arm64 ;;
  x86_64-apple-darwin)       export GOOS=darwin  GOARCH=amd64 ;;
  x86_64-unknown-linux-gnu)  export GOOS=linux   GOARCH=amd64 ;;
  aarch64-unknown-linux-gnu) export GOOS=linux   GOARCH=arm64 ;;
  x86_64-pc-windows-msvc)    export GOOS=windows GOARCH=amd64 ;;
  aarch64-pc-windows-msvc)   export GOOS=windows GOARCH=arm64 ;;
  *) echo "unmapped target triple: $TRIPLE" >&2; exit 1 ;;
esac

OUT="$ROOT/desktop/src-tauri/binaries/myaudit-serve-$TRIPLE$EXT"
mkdir -p "$(dirname "$OUT")"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$OUT" ./cmd/serve
chmod +x "$OUT"

echo "==> sidecar ready: ${OUT#"$ROOT"/}"
