#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/docker-compose.yml}"
DB_USER="${DB_USER:-game}"
DB_NAME="${DB_NAME:-game}"
DATABASE_URL="${DATABASE_URL:-postgres://game:game@localhost:5432/game?sslmode=disable}"
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
API_ADDR="${API_ADDR:-:8080}"
FRONTEND_HOST="${FRONTEND_HOST:-127.0.0.1}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
WORLD_ID="00000000-0000-0000-0000-000000000001"

usage() {
  cat >&2 <<'EOF'
Usage: ./scripts/dev.sh [normal|playtest|fast|SECONDS] [--reset]

Profiles:
  normal    14400 seconds/tick
  playtest  60 seconds/tick (default)
  fast      15 seconds/tick
  SECONDS   any positive integer duration

Options:
  --reset   recreate the disposable local database before starting services
EOF
}

fail_usage() {
  echo "$1" >&2
  usage
  exit 2
}

MODE="playtest"
TICK_DURATION_SECONDS=60
RESET=0
MODE_SET=0

for arg in "$@"; do
  case "$arg" in
    --reset)
      if (( RESET == 1 )); then
        fail_usage "--reset may only be provided once"
      fi
      RESET=1
      ;;
    -h|--help)
      usage >&1
      exit 0
      ;;
    normal)
      (( MODE_SET == 0 )) || fail_usage "only one tick profile may be provided"
      MODE="normal"
      TICK_DURATION_SECONDS=14400
      MODE_SET=1
      ;;
    playtest)
      (( MODE_SET == 0 )) || fail_usage "only one tick profile may be provided"
      MODE="playtest"
      TICK_DURATION_SECONDS=60
      MODE_SET=1
      ;;
    fast)
      (( MODE_SET == 0 )) || fail_usage "only one tick profile may be provided"
      MODE="fast"
      TICK_DURATION_SECONDS=15
      MODE_SET=1
      ;;
    *)
      [[ "$arg" =~ ^[1-9][0-9]*$ ]] || fail_usage "invalid tick profile or duration: $arg"
      (( MODE_SET == 0 )) || fail_usage "only one tick profile may be provided"
      MODE="custom"
      TICK_DURATION_SECONDS="$arg"
      MODE_SET=1
      ;;
  esac
done

psql_exec() {
  docker compose -f "$COMPOSE_FILE" exec -T postgres \
    psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" "$@"
}

wait_for_postgres() {
  echo "Waiting for PostgreSQL..."
  for _ in {1..30}; do
    if docker compose -f "$COMPOSE_FILE" exec -T postgres \
      pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
      echo "PostgreSQL is ready"
      return 0
    fi
    sleep 1
  done
  echo "PostgreSQL did not become ready after 30 seconds" >&2
  return 1
}

development_world_exists() {
  local table_exists world_exists
  table_exists="$(psql_exec -Atqc "SELECT to_regclass('public.worlds') IS NOT NULL;")"
  [[ "$table_exists" == "t" ]] || return 1
  world_exists="$(psql_exec -Atqc "SELECT EXISTS (SELECT 1 FROM worlds WHERE id='$WORLD_ID');")"
  [[ "$world_exists" == "t" ]]
}

ensure_development_database() {
  if development_world_exists; then
    echo "Using existing development world"
    return 0
  fi

  local table_exists
  table_exists="$(psql_exec -Atqc "SELECT to_regclass('public.worlds') IS NOT NULL;")"
  if [[ "$table_exists" == "t" ]]; then
    echo "Development schema exists without Bjornvik world; applying seed only"
    (cd "$BACKEND_DIR" && DEV_TICK_DURATION_SECONDS="$TICK_DURATION_SECONDS" ./scripts/bootstrap_db.sh --seed-only)
  else
    echo "Development database is empty; applying migrations and Bjornvik seed"
    (cd "$BACKEND_DIR" && DEV_TICK_DURATION_SECONDS="$TICK_DURATION_SECONDS" ./scripts/bootstrap_db.sh)
  fi
}

update_tick_schedule() {
  echo "Setting development world to $TICK_DURATION_SECONDS seconds/tick"
  psql_exec \
    -v tick_duration_seconds="$TICK_DURATION_SECONDS" \
    <<SQL
UPDATE worlds
SET tick_duration_seconds = :tick_duration_seconds,
    next_tick_at = now() + make_interval(secs => :tick_duration_seconds)
WHERE id = '$WORLD_ID';
SQL
}

API_PID=""
WORKER_PID=""
FRONTEND_PID=""

stop_process_group() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  kill -TERM -- "-$pid" 2>/dev/null || true
}

force_process_group_stop() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  kill -KILL -- "-$pid" 2>/dev/null || true
}

wait_for_process_group_exit() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  for _ in {1..20}; do
    if ! kill -0 -- "-$pid" 2>/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  force_process_group_stop "$pid"
}

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  stop_process_group "$API_PID"
  stop_process_group "$WORKER_PID"
  stop_process_group "$FRONTEND_PID"
  wait_for_process_group_exit "$API_PID"
  wait_for_process_group_exit "$WORKER_PID"
  wait_for_process_group_exit "$FRONTEND_PID"
  wait "$API_PID" "$WORKER_PID" "$FRONTEND_PID" 2>/dev/null || true
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

if (( RESET == 1 )); then
  echo "Resetting disposable local database"
  (cd "$BACKEND_DIR" && DEV_TICK_DURATION_SECONDS="$TICK_DURATION_SECONDS" ./scripts/reset_db.sh)
else
  docker compose -f "$COMPOSE_FILE" up -d postgres
  wait_for_postgres
  ensure_development_database
fi

wait_for_postgres
update_tick_schedule

echo
echo "Kinstead development environment"
echo
printf 'Mode:       %s\n' "$MODE"
printf 'Tick speed: %s seconds/tick\n' "$TICK_DURATION_SECONDS"
printf 'Database:   postgres://localhost:5432/%s\n' "$DB_NAME"
printf 'API:        %s\n' "$BACKEND_URL"
printf 'Frontend:   http://localhost:%s\n' "$FRONTEND_PORT"
printf 'Worker:     starting\n'
echo

(
  cd "$BACKEND_DIR"
  exec setsid env DATABASE_URL="$DATABASE_URL" API_ADDR="$API_ADDR" \
    go run -tags postgres ./cmd/api
) &
API_PID=$!

(
  cd "$BACKEND_DIR"
  exec setsid env DATABASE_URL="$DATABASE_URL" \
    go run -tags postgres ./cmd/worker
) &
WORKER_PID=$!

(
  cd "$FRONTEND_DIR"
  exec setsid env BACKEND_URL="$BACKEND_URL" \
    npm run dev -- --host "$FRONTEND_HOST" --port "$FRONTEND_PORT"
) &
FRONTEND_PID=$!

printf 'Worker:     running\n'
echo "Press Ctrl+C to stop. PostgreSQL will remain running."

set +e
wait -n -p EXITED_PID "$API_PID" "$WORKER_PID" "$FRONTEND_PID"
SERVICE_STATUS=$?
set -e

case "$EXITED_PID" in
  "$API_PID") SERVICE_NAME="API" ;;
  "$WORKER_PID") SERVICE_NAME="worker" ;;
  "$FRONTEND_PID") SERVICE_NAME="frontend" ;;
  *) SERVICE_NAME="development service" ;;
esac

if (( SERVICE_STATUS == 0 )); then
  echo "$SERVICE_NAME exited unexpectedly" >&2
  exit 1
fi
echo "$SERVICE_NAME exited with status $SERVICE_STATUS" >&2
exit "$SERVICE_STATUS"
