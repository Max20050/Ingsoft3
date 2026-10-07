#!/bin/sh
# Runs the backend suite with coverage and fails the build if the aggregate
# statement coverage falls below THRESHOLD. Go's own toolchain has no
# equivalent of coverlet's /p:Threshold, so this is the script the guide
# expects for stacks without one: run the tests, read go tool cover's total,
# compare it by hand.
#
# What counts: every package under internal/ except internal/migrate (applies
# embedded SQL files, no business rule to verify) and cmd/api (wires up
# routes and config — no rule of its own either, and if it's wrong the server
# just won't start). Both are the "arranque" the guide says to leave out.
set -eu

THRESHOLD=44
PROFILE=${COVERAGE_PROFILE:-coverage.out}

cd "$(dirname "$0")/.."
mkdir -p "$(dirname "$PROFILE")"

PACKAGES=$(go list ./... | grep -v '/cmd/api$' | grep -v '/internal/migrate$')

go test $PACKAGES -coverprofile="$PROFILE" -covermode=atomic

PCT=$(go tool cover -func="$PROFILE" | tail -1 | awk '{print $NF}' | tr -d '%')

echo "Total statement coverage: ${PCT}% (threshold: ${THRESHOLD}%)"
echo "NOTE: branch coverage is not reported — go tool cover has no branch mode;" \
     "the gate and this number are statement coverage only (see decisiones.md)."

# `sh` has no float arithmetic, so the comparison is done in awk.
if ! awk -v pct="$PCT" -v threshold="$THRESHOLD" 'BEGIN { exit (pct + 0 < threshold + 0) ? 1 : 0 }'; then
  echo "ERROR: Coverage for statements (${PCT}%) does not meet global threshold (${THRESHOLD}%)"
  exit 1
fi
