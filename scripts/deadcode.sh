#!/usr/bin/env bash
#
# Reachability gate.
#
# This repository has repeatedly grown subsystems that were complete, tested,
# and never called: validation, the cost governor's enforcing mode, lifecycle
# hooks, demand paging, the skill candidate pipeline, and the entire first-run
# welcome experience. Every one was found by a human deliberately going looking,
# which does not scale and did not catch them early. This turns that search into
# a build step.
#
# It is a ratchet, not a cleanup. Everything unreachable today is listed in the
# baseline; the build fails only on something NEW becoming unreachable. Removing
# an entry from the baseline (by deleting the dead code or wiring it up) is
# always safe.
#
# Known limitation: deadcode traces the call graph from main and from tests, so
# a service that is constructed, stored on a struct, and never invoked looks
# reachable. That is the exact shape of most of the instances above, so this
# gate narrows the problem rather than solving it. Manual review still matters.

set -euo pipefail

# Pinned so CI results do not drift with an upstream release.
DEADCODE_VERSION="v0.49.0"
BASELINE="$(dirname "$0")/../.deadcode-baseline"

# Line numbers are stripped: they change whenever a file is edited, and a gate
# that fails on unrelated edits gets disabled rather than fixed.
current="$(go run "golang.org/x/tools/cmd/deadcode@${DEADCODE_VERSION}" -test ./... 2>/dev/null \
  | sed -E 's/:[0-9]+:[0-9]+: unreachable func: /: /' \
  | sort -u)"

if [ ! -f "$BASELINE" ]; then
  echo "No baseline at $BASELINE. Writing the current state:"
  echo "$current"
  exit 1
fi

# Comments and blank lines are for humans; strip them before comparing.
baseline="$(grep -vE '^\s*(#|$)' "$BASELINE" | sort -u)"

new="$(comm -23 <(echo "$current") <(echo "$baseline") || true)"
fixed="$(comm -13 <(echo "$current") <(echo "$baseline") || true)"

status=0

if [ -n "$new" ]; then
  echo "Newly unreachable code:"
  echo "$new" | sed 's/^/  /'
  echo
  echo "Either wire it up, or delete it. If it is genuinely meant to be"
  echo "unreachable (an exported helper for future use, a design-system variant),"
  echo "add it to .deadcode-baseline with a comment saying why."
  status=1
fi

if [ -n "$fixed" ]; then
  echo "These baseline entries are now reachable or gone — remove them from"
  echo ".deadcode-baseline so the ratchet tightens:"
  echo "$fixed" | sed 's/^/  /'
  status=1
fi

if [ "$status" -eq 0 ]; then
  echo "Reachability unchanged ($(echo "$baseline" | wc -l | tr -d ' ') known entries)."
fi

exit "$status"
