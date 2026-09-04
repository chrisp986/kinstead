#!/usr/bin/env bash
set -euo pipefail

# Development-only bootstrap: this reapplies the complete SQL stream and then
# loads the demo seed. Persistent environments should track applied migrations
# with Goose instead, for example:
#   goose -dir db/migrations postgres "$DATABASE_URL" up
# A future db-migrate command should be the only migration path for staging or
# production; use reset_db.sh only for disposable local databases.

COMPOSE_FILE="${COMPOSE_FILE:-../docker-compose.yml}"
DB_USER="${DB_USER:-game}"
DB_NAME="${DB_NAME:-game}"

if [[ -v DEV_TICK_DURATION_SECONDS && -z "$DEV_TICK_DURATION_SECONDS" ]]; then
  echo "DEV_TICK_DURATION_SECONDS must be a positive integer" >&2
  exit 1
fi

DEV_TICK_DURATION_SECONDS="${DEV_TICK_DURATION_SECONDS:-14400}"

if ! [[ "$DEV_TICK_DURATION_SECONDS" =~ ^[1-9][0-9]*$ ]]; then
  echo "DEV_TICK_DURATION_SECONDS must be a positive integer" >&2
  exit 1
fi

for migration in db/migrations/*.sql; do
  echo "Applying $migration"
  awk '/^-- \+goose Down/{exit} {print}' "$migration" \
    | docker compose -f "$COMPOSE_FILE" exec -T postgres \
        psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME"
done

echo "Applying development seed"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql \
  -v ON_ERROR_STOP=1 \
  -v tick_duration_seconds="$DEV_TICK_DURATION_SECONDS" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  < db/seeds/dev_bjornvik.sql
