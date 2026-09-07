#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEV_SESSION_SCRIPT="$SCRIPT_DIR/dev_session.sh"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT
mkdir -p "$TEMP_DIR/scripts" "$TEMP_DIR/backend" "$TEMP_DIR/bin"
cp "$DEV_SESSION_SCRIPT" "$TEMP_DIR/scripts/dev_session.sh"
chmod +x "$TEMP_DIR/scripts/dev_session.sh"

cat > "$TEMP_DIR/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ "$*" == *'exec -T postgres psql -X -Atq -v ON_ERROR_STOP=1'* ]] || exit 2
cat > "$MOCK_SQL"
if [[ "${MOCK_DB_FAIL:-0}" == 1 ]]; then
  echo 'mock database failure' >&2
  exit 1
fi
printf '%s\n' "${MOCK_PLAYER_ID:-}"
MOCK
cat > "$TEMP_DIR/bin/go" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_GO_CALLS"
[[ "$PWD" == "$MOCK_BACKEND_DIR" ]] || exit 2
[[ "${DATABASE_URL:-}" == "$MOCK_DATABASE_URL" ]] || exit 2
if [[ "${MOCK_GO_FAIL:-0}" == 1 ]]; then
  echo 'mock session provisioning failure' >&2
  exit 1
fi
printf '%s\n' 'fixture-session-key-not-a-real-secret'
MOCK
chmod +x "$TEMP_DIR/bin/docker" "$TEMP_DIR/bin/go"
export PATH="$TEMP_DIR/bin:$PATH"
export MOCK_SQL="$TEMP_DIR/sql" MOCK_GO_CALLS="$TEMP_DIR/go-calls"
export MOCK_BACKEND_DIR="$TEMP_DIR/backend"
export MOCK_DATABASE_URL='postgres://test:test@localhost:5432/test?sslmode=disable'
export DATABASE_URL="$MOCK_DATABASE_URL"
export COMPOSE_FILE="$TEMP_DIR/docker-compose.yml"
export MOCK_PLAYER_ID='11111111-1111-4111-8111-111111111111'

run_success() {
  : > "$MOCK_GO_CALLS"
  "$TEMP_DIR/scripts/dev_session.sh" > "$TEMP_DIR/output" 2> "$TEMP_DIR/error"
  grep -Fx "Player:      $MOCK_PLAYER_ID" "$TEMP_DIR/output" >/dev/null
  grep -Fx 'Session key: fixture-session-key-not-a-real-secret' "$TEMP_DIR/output" >/dev/null
  grep -F -- "-player-id $MOCK_PLAYER_ID -lifetime ${DEV_SESSION_LIFETIME:-24h}" "$MOCK_GO_CALLS" >/dev/null
  [[ "$(wc -l < "$MOCK_GO_CALLS")" -eq 1 ]]
}

# A fresh seed and an existing owner both resolve to the actual owner UUID.
run_success
[[ "$(grep -c 'owner_player_id IS NULL' "$MOCK_SQL")" -eq 2 ]]
grep -F 'ON CONFLICT (external_auth_subject) DO NOTHING' "$MOCK_SQL" >/dev/null
grep -F 'SELECT owner_player_id::text FROM households' "$MOCK_SQL" >/dev/null
! grep -E 'DELETE FROM|TRUNCATE|DROP TABLE' "$MOCK_SQL" >/dev/null

# A custom lifetime reaches the authoritative CLI unchanged.
DEV_SESSION_LIFETIME=2h run_success

# A missing or invalid owner must not issue a credential.
for MOCK_PLAYER_ID in '' 'not-a-uuid'; do
  export MOCK_PLAYER_ID
  : > "$MOCK_GO_CALLS"
  if "$TEMP_DIR/scripts/dev_session.sh" > "$TEMP_DIR/output" 2> "$TEMP_DIR/error"; then
    echo 'issued a session without a valid owner' >&2
    exit 1
  fi
  [[ ! -s "$MOCK_GO_CALLS" ]]
  ! grep -F 'Session key:' "$TEMP_DIR/output" >/dev/null
  grep -F 'no session was issued' "$TEMP_DIR/error" >/dev/null
done
export MOCK_PLAYER_ID='11111111-1111-4111-8111-111111111111'

# Database errors must stop before invoking the session CLI.
: > "$MOCK_GO_CALLS"
if MOCK_DB_FAIL=1 "$TEMP_DIR/scripts/dev_session.sh" > "$TEMP_DIR/output" 2> "$TEMP_DIR/error"; then
  echo 'ignored database failure' >&2
  exit 1
fi
[[ ! -s "$MOCK_GO_CALLS" ]]

# Provisioning errors must fail startup rather than claim a successful key.
: > "$MOCK_GO_CALLS"
if MOCK_GO_FAIL=1 "$TEMP_DIR/scripts/dev_session.sh" > "$TEMP_DIR/output" 2> "$TEMP_DIR/error"; then
  echo 'ignored session provisioning failure' >&2
  exit 1
fi
[[ "$(wc -l < "$MOCK_GO_CALLS")" -eq 1 ]]
! grep -F 'fixture-session-key-not-a-real-secret' "$TEMP_DIR/output" >/dev/null
! grep -F 'Paste the key' "$TEMP_DIR/output" >/dev/null

bash -n "$DEV_SESSION_SCRIPT"
echo 'development session helper tests passed'
