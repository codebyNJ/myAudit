#!/usr/bin/env bash
# Fail if Go statement coverage drops below a floor.
#
# A floor, not a target. It sits just under the measured value so an ordinary
# refactor that shifts a few statements does not fail the build, while a real
# regression does. Raise MIN as coverage climbs — that is the ratchet.
#
# Usage: scripts/coverage-gate.sh [profile] [min-percent]
set -euo pipefail

PROFILE="${1:-coverage.out}"
MIN="${2:-50}"

if [ ! -f "$PROFILE" ]; then
  echo "coverage profile not found: $PROFILE" >&2
  exit 1
fi

TOTAL_LINE="$(go tool cover -func="$PROFILE" | tail -1)"
echo "$TOTAL_LINE"

PCT="$(printf '%s\n' "$TOTAL_LINE" | awk '{print $NF}' | tr -d '%')"
if [ -z "$PCT" ]; then
  echo "could not parse coverage percentage from: $TOTAL_LINE" >&2
  exit 1
fi

if awk -v p="$PCT" -v m="$MIN" 'BEGIN { exit !(p + 0 < m + 0) }'; then
  echo "FAIL: Go coverage ${PCT}% is below the ${MIN}% floor" >&2
  exit 1
fi

echo "OK: Go coverage ${PCT}% (floor ${MIN}%)"
