#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -v DEV_TICK_DURATION_SECONDS && -z "$DEV_TICK_DURATION_SECONDS" ]]; then
  echo "DEV_TICK_DURATION_SECONDS must be a positive integer" >&2
  exit 1
fi

export DEV_TICK_DURATION_SECONDS="${DEV_TICK_DURATION_SECONDS:-60}"

exec "$SCRIPT_DIR/reset_db.sh"
