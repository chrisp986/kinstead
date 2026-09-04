#!/usr/bin/env bash
set -euo pipefail

# Development-only bootstrap: apply tracked migrations and load the demo seed.
# The seed is intentionally separate from migrations and must only be applied
# to a fresh database or when the development world is absent.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT_DIR="$(cd "$BACKEND_DIR/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/docker-compose.yml}"
DB_USER="${DB_USER:-game}"
DB_NAME="${DB_NAME:-game}"
DATABASE_URL="${DATABASE_URL:-postgres://game:game@localhost:5432/game?sslmode=disable}"

if [[ "${1:-}" != "" && "${1:-}" != "--seed-only" ]] || [[ "$#" -gt 1 ]]; then
  echo "usage: $0 [--seed-only]" >&2
  exit 1
fi

if [[ -v DEV_TICK_DURATION_SECONDS && -z "$DEV_TICK_DURATION_SECONDS" ]]; then
  echo "DEV_TICK_DURATION_SECONDS must be a positive integer" >&2
  exit 1
fi

DEV_TICK_DURATION_SECONDS="${DEV_TICK_DURATION_SECONDS:-14400}"

if ! [[ "$DEV_TICK_DURATION_SECONDS" =~ ^[1-9][0-9]*$ ]]; then
  echo "DEV_TICK_DURATION_SECONDS must be a positive integer" >&2
  exit 1
fi

if [[ "${1:-}" != "--seed-only" ]]; then
  echo "Applying tracked migrations"
  COMPOSE_FILE="$COMPOSE_FILE" DB_USER="$DB_USER" DB_NAME="$DB_NAME" \
    DATABASE_URL="$DATABASE_URL" "$SCRIPT_DIR/migrate_db.sh"
fi

echo "Applying development seed"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql \
  -v ON_ERROR_STOP=1 \
  -v tick_duration_seconds="$DEV_TICK_DURATION_SECONDS" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  < "$BACKEND_DIR/db/seeds/dev_bjornvik.sql"
