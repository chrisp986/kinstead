#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT_DIR="$(cd "$BACKEND_DIR/.." && pwd)"

COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/docker-compose.yml}"
DB_USER="${DB_USER:-game}"
DB_NAME="${DB_NAME:-game}"
DATABASE_URL="${DATABASE_URL:-postgres://game:game@localhost:5432/game?sslmode=disable}"
DEV_TICK_DURATION_SECONDS="${DEV_TICK_DURATION_SECONDS:-14400}"

if ! [[ "$DEV_TICK_DURATION_SECONDS" =~ ^[1-9][0-9]*$ ]]; then
  echo "DEV_TICK_DURATION_SECONDS must be a positive integer" >&2
  exit 1
fi

docker compose -f "$COMPOSE_FILE" down -v
docker compose -f "$COMPOSE_FILE" up -d postgres

echo "Waiting for PostgreSQL..."
for _ in $(seq 1 30); do
  if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
    COMPOSE_FILE="$COMPOSE_FILE" DB_USER="$DB_USER" DB_NAME="$DB_NAME" \
      DATABASE_URL="$DATABASE_URL" DEV_TICK_DURATION_SECONDS="$DEV_TICK_DURATION_SECONDS" \
      "$BACKEND_DIR/scripts/bootstrap_db.sh"
    exit 0
  fi
  sleep 1
done

echo "PostgreSQL did not become ready" >&2
exit 1
