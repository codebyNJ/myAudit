#!/usr/bin/env bash
# Ensure Pillow is available (PEP 668–safe venv). Prints the python executable path.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VENV="$ROOT/.venv-dmg"

if [ ! -d "$VENV" ]; then
  python3 -m venv "$VENV"
fi

if ! "$VENV/bin/python" -c "import PIL" 2>/dev/null; then
  "$VENV/bin/pip" install -q pillow
fi

echo "$VENV/bin/python"
