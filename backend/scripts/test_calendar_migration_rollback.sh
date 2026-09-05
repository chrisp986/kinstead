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

run_goose() {
  (cd "$BACKEND_DIR" && go run "github.com/pressly/goose/v3/cmd/goose@$GOOSE_VERSION" \
    -dir db/migrations postgres "$DATABASE_URL" "$@")
}

assert_catalog_state() {
  local expected="$1" relation trigger function actual
  local -a compatibility=(
    "chronicle_entries:chronicle_entries_fill_game_day:game_fill_chronicle_game_day"
    "shipments:shipments_fill_game_days:game_fill_shipment_game_days"
    "contract_obligations:contract_obligations_fill_game_days:game_fill_obligation_game_days"
    "contracts:contracts_fill_game_days:game_fill_contract_game_days"
    "relationship_events:relationship_events_fill_game_day:game_fill_relationship_game_day"
    "household_decisions:household_decisions_fill_game_days:game_fill_decision_game_days"
  )

  for entry in "${compatibility[@]}"; do
    IFS=: read -r relation trigger function <<<"$entry"
    actual="$(psql_for_url "$DATABASE_URL" -Atqc "
      SELECT EXISTS (
        SELECT 1
        FROM pg_trigger t
        JOIN pg_class c ON c.oid = t.tgrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = '$relation'
          AND t.tgname = '$trigger'
          AND NOT t.tgisinternal
      )
      AND to_regprocedure('public.$function()') IS NOT NULL;
    ")"
    if [[ "$actual" != "$expected" ]]; then
      echo "compatibility state mismatch for $relation/$trigger/$function: got $actual, want $expected" >&2
      return 1
    fi
  done
}

run_goose down
assert_catalog_state t
run_goose up
assert_catalog_state f

echo "calendar migration rollback round-trip passed"
