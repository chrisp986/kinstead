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
Usage: ./scripts/dev.sh [normal|playtest|fast|SECONDS] [--reset] [--print-config]

Profiles:
  normal    14400 seconds/tick
  playtest  60 seconds/tick (default)
  fast      15 seconds/tick
  SECONDS   any positive integer duration

Options:
  --reset   recreate the disposable local database before starting services
  --print-config  print the parsed configuration without touching Docker
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
PRINT_CONFIG=0

for arg in "$@"; do
  case "$arg" in
    --reset)
      if (( RESET == 1 )); then
        fail_usage "--reset may only be provided once"
      fi
      RESET=1
      ;;
    --print-config)
      if (( PRINT_CONFIG == 1 )); then
        fail_usage "--print-config may only be provided once"
      fi
      PRINT_CONFIG=1
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

if (( PRINT_CONFIG == 1 )); then
  printf 'mode=%s\n' "$MODE"
  printf 'tick_duration_seconds=%s\n' "$TICK_DURATION_SECONDS"
  printf 'reset=%s\n' "$RESET"
  exit 0
fi

if (( BASH_VERSINFO[0] < 5 || (BASH_VERSINFO[0] == 5 && BASH_VERSINFO[1] < 1) )); then
  echo "scripts/dev.sh requires Bash >= 5.1 on Linux or WSL" >&2
  exit 1
fi

for command_name in docker go npm curl pgrep ps setsid; do
  command -v "$command_name" >/dev/null 2>&1 || {
    echo "scripts/dev.sh requires '$command_name'" >&2
    exit 1
  }
done
docker compose version >/dev/null 2>&1 || {
  echo "scripts/dev.sh requires Docker Compose v2" >&2
  exit 1
}

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
  [[ "$(psql_exec -Atqc "SELECT EXISTS (SELECT 1 FROM worlds WHERE id='$WORLD_ID');")" == "t" ]]
}

apply_database_migrations() {
  echo "Applying pending database migrations"
  DATABASE_URL="$DATABASE_URL" COMPOSE_FILE="$COMPOSE_FILE" \
    DB_USER="$DB_USER" DB_NAME="$DB_NAME" \
    "$BACKEND_DIR/scripts/migrate_db.sh"
}

ensure_development_world() {
  if development_world_exists; then
    echo "Using existing development world"
    return 0
  fi

  echo "Development world is missing; applying Bjornvik seed"
  DEV_TICK_DURATION_SECONDS="$TICK_DURATION_SECONDS" COMPOSE_FILE="$COMPOSE_FILE" \
    DB_USER="$DB_USER" DB_NAME="$DB_NAME" DATABASE_URL="$DATABASE_URL" \
    "$BACKEND_DIR/scripts/bootstrap_db.sh" --seed-only
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
API_WRAPPER_PID=""
WORKER_WRAPPER_PID=""
FRONTEND_WRAPPER_PID=""
LAST_WRAPPER_PID=""
LAST_PROCESS_GROUP_PID=""

launch_service() {
  local workdir="$1"
  shift
  local child_pid=""

  (
    cd "$workdir"
    exec setsid --wait "$@"
  ) &
  LAST_WRAPPER_PID=$!
  LAST_PROCESS_GROUP_PID="$LAST_WRAPPER_PID"

  # setsid forks when its caller is already a process-group leader. Track the
  # forked child so readiness checks and cleanup target the complete service
  # process group, while the wrapper remains available for wait -n.
  for _ in {1..50}; do
    child_pid="$(pgrep -P "$LAST_WRAPPER_PID" | head -n 1 || true)"
    if [[ -n "$child_pid" ]]; then
      LAST_PROCESS_GROUP_PID="$(ps -o pgid= -p "$child_pid" | tr -d ' ')"
      break
    fi
    if ! kill -0 "$LAST_WRAPPER_PID" 2>/dev/null; then
      break
    fi
    sleep 0.1
  done

  if [[ -z "$LAST_PROCESS_GROUP_PID" ]]; then
    LAST_PROCESS_GROUP_PID="$(ps -o pgid= -p "$LAST_WRAPPER_PID" | tr -d ' ')"
  fi
}

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

stop_process() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  kill -TERM "$pid" 2>/dev/null || true
}

force_process_stop() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  kill -KILL "$pid" 2>/dev/null || true
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

# cleanup is invoked indirectly through traps; ShellCheck cannot follow that
# control flow and otherwise reports SC2317 for the cleanup commands.
# shellcheck disable=SC2317
cleanup() {
  local status=$?
  trap - EXIT INT TERM
  stop_process_group "$API_PID"
  stop_process_group "$WORKER_PID"
  stop_process_group "$FRONTEND_PID"
  wait_for_process_group_exit "$API_PID"
  wait_for_process_group_exit "$WORKER_PID"
  wait_for_process_group_exit "$FRONTEND_PID"
  stop_process "$API_WRAPPER_PID"
  stop_process "$WORKER_WRAPPER_PID"
  stop_process "$FRONTEND_WRAPPER_PID"
  for wrapper_pid in "$API_WRAPPER_PID" "$WORKER_WRAPPER_PID" "$FRONTEND_WRAPPER_PID"; do
    [[ -n "$wrapper_pid" ]] || continue
    for _ in {1..20}; do
      if ! kill -0 "$wrapper_pid" 2>/dev/null; then
        break
      fi
      sleep 0.1
    done
    if kill -0 "$wrapper_pid" 2>/dev/null; then
      force_process_stop "$wrapper_pid"
    fi
  done
  wait "$API_WRAPPER_PID" "$WORKER_WRAPPER_PID" "$FRONTEND_WRAPPER_PID" 2>/dev/null || true
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

if (( RESET == 1 )); then
  echo "Resetting disposable local database"
  DEV_TICK_DURATION_SECONDS="$TICK_DURATION_SECONDS" COMPOSE_FILE="$COMPOSE_FILE" \
    DB_USER="$DB_USER" DB_NAME="$DB_NAME" DATABASE_URL="$DATABASE_URL" \
    "$BACKEND_DIR/scripts/reset_db.sh"
else
  docker compose -f "$COMPOSE_FILE" up -d postgres
  wait_for_postgres
fi

wait_for_postgres
apply_database_migrations
ensure_development_world
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

launch_service "$BACKEND_DIR" env DATABASE_URL="$DATABASE_URL" API_ADDR="$API_ADDR" \
  go run -tags postgres ./cmd/api
API_WRAPPER_PID="$LAST_WRAPPER_PID"
API_PID="$LAST_PROCESS_GROUP_PID"

launch_service "$BACKEND_DIR" env DATABASE_URL="$DATABASE_URL" \
  go run -tags postgres ./cmd/worker
WORKER_WRAPPER_PID="$LAST_WRAPPER_PID"
WORKER_PID="$LAST_PROCESS_GROUP_PID"

launch_service "$FRONTEND_DIR" env BACKEND_URL="$BACKEND_URL" \
  npm run dev -- --host "$FRONTEND_HOST" --port "$FRONTEND_PORT"
FRONTEND_WRAPPER_PID="$LAST_WRAPPER_PID"
FRONTEND_PID="$LAST_PROCESS_GROUP_PID"

wait_for_service() {
  local name="$1" pid="$2" url="$3"
  for _ in {1..60}; do
    if ! kill -0 -- "-$pid" 2>/dev/null; then
      echo "$name exited before becoming ready" >&2
      return 1
    fi
    if curl --fail --silent --show-error --max-time 1 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done
  echo "$name did not become ready after 12 seconds" >&2
  return 1
}

wait_for_service "API" "$API_PID" "$BACKEND_URL/healthz"
wait_for_service "frontend" "$FRONTEND_PID" "http://127.0.0.1:$FRONTEND_PORT/"
if ! kill -0 -- "-$WORKER_PID" 2>/dev/null; then
  echo "worker exited before becoming ready" >&2
  exit 1
fi

printf 'Worker:     running\n'
echo "Development environment ready"
echo "Press Ctrl+C to stop. PostgreSQL will remain running."

set +e
wait -n -p EXITED_PID "$API_WRAPPER_PID" "$WORKER_WRAPPER_PID" "$FRONTEND_WRAPPER_PID"
SERVICE_STATUS=$?
set -e

case "$EXITED_PID" in
  "$API_WRAPPER_PID") SERVICE_NAME="API" ;;
  "$WORKER_WRAPPER_PID") SERVICE_NAME="worker" ;;
  "$FRONTEND_WRAPPER_PID") SERVICE_NAME="frontend" ;;
  *) SERVICE_NAME="development service" ;;
esac

if (( SERVICE_STATUS == 0 )); then
  echo "$SERVICE_NAME exited unexpectedly" >&2
  exit 1
fi
echo "$SERVICE_NAME exited with status $SERVICE_STATUS" >&2
exit "$SERVICE_STATUS"
