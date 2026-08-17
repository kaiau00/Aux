#!/usr/bin/env bash
#
# Coverage ratchet.
#
# Total statement coverage must not fall below .coverage-floor. The floor starts
# at whatever the tree already achieves rather than at a target, so the gate can
# be switched on today: it stops coverage regressing without blocking every
# change until a distant number is met.
#
# When coverage rises meaningfully above the floor, this says so and asks for the
# floor to be raised. That is the ratchet — the same shape as
# scripts/deadcode.sh — and it is what turns a number nobody looks at into one
# that only moves in one direction.
#
# Note the floor is well under the 80% this project states as its bar. The gap is
# real and the gate does not close it; it only stops it widening.

set -euo pipefail

cd "$(dirname "$0")/.."
FLOOR_FILE=".coverage-floor"

# Icons are chosen at package init from the ambient locale, and the golden
# snapshots record the Unicode rendering; pin it so coverage runs cannot fail
# for a reason unrelated to coverage.
export AUX_UNICODE_ICONS=1

# Plain mktemp: BSD (macOS) accepts a bare -t prefix, GNU requires X's in the
# template, and no-argument mktemp is the form both agree on.
profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT

if ! go test -coverprofile="$profile" ./... >/dev/null 2>&1; then
  echo "Tests failed; coverage not measured."
  exit 1
fi

total="$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/,"",$NF); print $NF}')"
if [ -z "$total" ]; then
  echo "Could not read total coverage from the profile."
  exit 1
fi

if [ ! -f "$FLOOR_FILE" ]; then
  echo "No floor recorded. Current total coverage is ${total}%."
  echo "Write it to $FLOOR_FILE to start the ratchet."
  exit 1
fi
floor="$(tr -d '[:space:]' < "$FLOOR_FILE")"

# awk rather than bash arithmetic: these are decimals.
below="$(awk -v t="$total" -v f="$floor" 'BEGIN { print (t < f) ? 1 : 0 }')"
slack="$(awk -v t="$total" -v f="$floor" 'BEGIN { printf "%.1f", t - f }')"

if [ "$below" -eq 1 ]; then
  echo "Coverage ${total}% is below the floor of ${floor}%."
  echo
  echo "Add tests for what this change touched, or -- if the drop is genuinely"
  echo "expected, such as deleting well-tested code -- lower $FLOOR_FILE"
  echo "deliberately and say why in the commit message."
  exit 1
fi

echo "Coverage ${total}% (floor ${floor}%, +${slack})."

# Raising the floor is the point; prompt for it once the gap is real rather than
# noise from a handful of statements.
if [ "$(awk -v s="$slack" 'BEGIN { print (s >= 2) ? 1 : 0 }')" -eq 1 ]; then
  echo
  echo "Coverage is ${slack} points above the floor. Raise $FLOOR_FILE to ${total}"
  echo "so the gain is held."
fi
