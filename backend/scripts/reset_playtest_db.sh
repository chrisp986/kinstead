#!/usr/bin/env bash
set -euo pipefail

if [[ -v DEV_TICK_DURATION_SECONDS && -z "$DEV_TICK_DURATION_SECONDS" ]]; then
  echo "DEV_TICK_DURATION_SECONDS must be a positive integer" >&2
  exit 1
fi

export DEV_TICK_DURATION_SECONDS="${DEV_TICK_DURATION_SECONDS:-60}"

exec ./scripts/reset_db.sh
