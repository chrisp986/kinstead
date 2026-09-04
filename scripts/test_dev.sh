#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEV_SCRIPT="$SCRIPT_DIR/dev.sh"

assert_config() {
  local input="$1" expected_mode="$2" expected_duration="$3" output
  output="$("$DEV_SCRIPT" "$input" --print-config)"
  grep -Fx "mode=$expected_mode" <<<"$output" >/dev/null
  grep -Fx "tick_duration_seconds=$expected_duration" <<<"$output" >/dev/null
  grep -Fx "reset=0" <<<"$output" >/dev/null
}

assert_config normal normal 14400
assert_config playtest playtest 60
assert_config fast fast 15
assert_config 30 custom 30
assert_config 120 custom 120

default_output="$("$DEV_SCRIPT" --print-config)"
grep -Fx 'mode=playtest' <<<"$default_output" >/dev/null
grep -Fx 'tick_duration_seconds=60' <<<"$default_output" >/dev/null

for invalid in 0 -1 abc 1.5 unknown; do
  if "$DEV_SCRIPT" "$invalid" --print-config >/dev/null 2>&1; then
    echo "accepted invalid launcher argument: $invalid" >&2
    exit 1
  fi
done

if "$DEV_SCRIPT" fast playtest --print-config >/dev/null 2>&1; then
  echo "accepted duplicate tick profiles" >&2
  exit 1
fi

if "$DEV_SCRIPT" --reset --reset --print-config >/dev/null 2>&1; then
  echo "accepted duplicate --reset options" >&2
  exit 1
fi

echo "development launcher argument tests passed"
