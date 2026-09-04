#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DATABASE_URL="${TEST_DATABASE_URL:-postgres://game:game@localhost:5432/game?sslmode=disable}"
COMPOSE_FILE="${COMPOSE_FILE:-$BACKEND_DIR/../docker-compose.yml}"
DB_USER="${DB_USER:-game}"
GOOSE_VERSION="v3.26.0"

psql_for_url() {
  local url="$1" database
  shift
  if psql "$url" -Atqc 'SELECT 1' >/dev/null 2>&1; then
    psql "$url" "$@"
    return
  fi
  database="${url%%\?*}"
  database="${database##*/}"
  docker compose -f "$COMPOSE_FILE" exec -T postgres \
    psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$database" "$@"
}

base_url="${DATABASE_URL%%\?*}"
query_suffix=""
if [[ "$DATABASE_URL" == *\?* ]]; then
  query_suffix="?${DATABASE_URL#*\?}"
fi
admin_url="${base_url%/*}/postgres${query_suffix}"

suffix="${BASHPID}_${RANDOM}"
upgrade_db="kinstead_game_time_upgrade_${suffix}"
fresh_db="kinstead_game_time_fresh_${suffix}"
upgrade_url="${base_url%/*}/${upgrade_db}${query_suffix}"
fresh_url="${base_url%/*}/${fresh_db}${query_suffix}"
legacy_dir="$(mktemp -d)"

# cleanup is invoked indirectly through EXIT; ShellCheck cannot follow that
# control flow and otherwise reports SC2317 for the cleanup commands.
# shellcheck disable=SC2317
cleanup() {
  psql_for_url "$admin_url" -c "DROP DATABASE IF EXISTS $upgrade_db" >/dev/null 2>&1 || true
  psql_for_url "$admin_url" -c "DROP DATABASE IF EXISTS $fresh_db" >/dev/null 2>&1 || true
  rm -rf "$legacy_dir"
}
trap cleanup EXIT

create_database() {
  local database_name="$1"
  [[ "$database_name" =~ ^[a-z0-9_]+$ ]] || {
    echo "generated database name is unsafe" >&2
    exit 1
  }
  psql_for_url "$admin_url" -c "CREATE DATABASE $database_name" >/dev/null
}

run_migrations() {
  local database_url="$1" migrations_dir="$2"
  DATABASE_URL="$database_url" MIGRATIONS_DIR="$migrations_dir" \
    GOOSE_VERSION="$GOOSE_VERSION" "$BACKEND_DIR/scripts/migrate_db.sh"
}

create_database "$upgrade_db"
create_database "$fresh_db"

# Build a pre-game-time schema from the immutable migration history, then add
# the same logical Bjornvik world/characters that the current seed creates.
for migration in "$BACKEND_DIR"/db/migrations/0000{01..13}_*.sql; do
  cp "$migration" "$legacy_dir/"
done
run_migrations "$upgrade_url" "$legacy_dir"
psql_for_url "$upgrade_url" <<'SQL'
INSERT INTO worlds (id, name, historical_start_date, current_tick, tick_duration_seconds, next_tick_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'Development World', DATE '0980-01-01', 0, 60, now());
INSERT INTO locations (id, world_id, name, location_type)
VALUES
  ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'Bjornvik', 'farm'),
  ('00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'Hrafnstead', 'farm');
INSERT INTO households (id, world_id, location_id, name, specialization, created_tick)
VALUES
  ('00000000-0000-0000-0000-000000000020', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000010', 'Bjornvik', 'fishing', 0),
  ('00000000-0000-0000-0000-000000000021', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000011', 'Hrafnstead', 'agriculture', 0);
INSERT INTO characters (id, household_id, name, birth_date, labor_capacity_milli)
VALUES
  ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000020', 'Bjorn', DATE '0948-01-01', 1000),
  ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000020', 'Astrid', DATE '0951-01-01', 1000),
  ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000020', 'Einar', DATE '0963-01-01', 1000),
  ('00000000-0000-0000-0000-000000000104', '00000000-0000-0000-0000-000000000020', 'Ragnhild', DATE '0967-01-01', 500),
  ('00000000-0000-0000-0000-000000000105', '00000000-0000-0000-0000-000000000020', 'Sven', DATE '0974-01-01', 0);
INSERT INTO shipments (
  id, world_id, sender_household_id, receiver_household_id,
  origin_location_id, destination_location_id, resource_code,
  quantity_milli, departure_tick, expected_arrival_tick, transport_cost_milli, status
) VALUES (
  '00000000-0000-0000-0000-000000000301',
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000021',
  '00000000-0000-0000-0000-000000000020',
  '00000000-0000-0000-0000-000000000011',
  '00000000-0000-0000-0000-000000000010',
  'provisions', 30000, 0, 2, 1000, 'in_transit'
);
SQL
run_migrations "$upgrade_url" "$BACKEND_DIR/db/migrations"

run_migrations "$fresh_url" "$BACKEND_DIR/db/migrations"
psql_for_url "$fresh_url" -v tick_duration_seconds=60 \
  -f "$BACKEND_DIR/db/seeds/dev_bjornvik.sql" >/dev/null

upgrade_world="$(psql_for_url "$upgrade_url" -Atqc '
  SELECT concat_ws(chr(124), current_game_day, calendar_remainder, game_days_per_tick_num, game_days_per_tick_den, setting_start_year)
  FROM worlds WHERE id = '\''00000000-0000-0000-0000-000000000001'\'';
')"
fresh_world="$(psql_for_url "$fresh_url" -Atqc '
  SELECT concat_ws(chr(124), current_game_day, calendar_remainder, game_days_per_tick_num, game_days_per_tick_den, setting_start_year)
  FROM worlds WHERE id = '\''00000000-0000-0000-0000-000000000001'\'';
')"
test "$upgrade_world" = "$fresh_world"

upgrade_characters="$(psql_for_url "$upgrade_url" -Atqc '
  SELECT name || chr(124) || birth_game_day FROM characters ORDER BY name;
')"
fresh_characters="$(psql_for_url "$fresh_url" -Atqc '
  SELECT name || chr(124) || birth_game_day FROM characters ORDER BY name;
')"
test "$upgrade_characters" = "$fresh_characters"

upgrade_shipments="$(psql_for_url "$upgrade_url" -Atqc '
  SELECT id || chr(124) || departure_game_day || chr(124) || expected_arrival_game_day
  FROM shipments ORDER BY id;
')"
fresh_shipments="$(psql_for_url "$fresh_url" -Atqc '
  SELECT id || chr(124) || departure_game_day || chr(124) || expected_arrival_game_day
  FROM shipments ORDER BY id;
')"
test "$upgrade_shipments" = "$fresh_shipments"

echo "legacy/fresh game-time migration equivalence passed"
