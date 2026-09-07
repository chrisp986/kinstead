#!/usr/bin/env bash
# Local development only. This is an operator provisioning command, not an
# authentication bypass or an endpoint that can claim a household.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/docker-compose.yml}"
DB_USER="${DB_USER:-game}"
DB_NAME="${DB_NAME:-game}"
DATABASE_URL="${DATABASE_URL:-postgres://game:game@localhost:5432/game?sslmode=disable}"
DEV_SESSION_LIFETIME="${DEV_SESSION_LIFETIME:-24h}"

# Use the existing owner if the developer has already provisioned one. A fresh
# seed is unowned, so create a dedicated local player and claim Bjornvik only
# while it remains unowned. Never replace another player's ownership.
player_id="$(docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql -X -Atq -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" <<'SQL'
BEGIN;
INSERT INTO players (external_auth_subject)
SELECT 'local-development-bjornvik'
WHERE EXISTS (
    SELECT 1 FROM households
    WHERE id = '00000000-0000-0000-0000-000000000020'
      AND world_id = '00000000-0000-0000-0000-000000000001'
      AND owner_player_id IS NULL
)
ON CONFLICT (external_auth_subject) DO NOTHING;
UPDATE households
SET owner_player_id = (
    SELECT id FROM players
    WHERE external_auth_subject = 'local-development-bjornvik'
)
WHERE id = '00000000-0000-0000-0000-000000000020'
  AND world_id = '00000000-0000-0000-0000-000000000001'
  AND owner_player_id IS NULL;
SELECT owner_player_id::text FROM households
WHERE id = '00000000-0000-0000-0000-000000000020'
  AND world_id = '00000000-0000-0000-0000-000000000001';
COMMIT;
SQL
)"

if [[ ! "$player_id" =~ ^[0-9a-fA-F-]{36}$ ]]; then
  echo "Could not resolve the Bjornvik development player; no session was issued." >&2
  exit 1
fi

printf '\nLocal development session\n'
printf 'Player:      %s\n' "$player_id"
printf 'Household:   Bjornvik\n'
printf 'Session key: '
# The existing CLI generates a random 256-bit secret, stores only its hash,
# and prints the secret once. Do not capture it in a shell variable, pass it
# as an argument, or write it to a file. The CLI enforces the 30-day maximum.
(
  cd "$BACKEND_DIR"
  DATABASE_URL="$DATABASE_URL" go run -tags postgres ./cmd/session \
    -player-id "$player_id" -lifetime "$DEV_SESSION_LIFETIME"
)
printf 'Paste the key into /sign-in. It expires in %s. Keep it private.\n' "$DEV_SESSION_LIFETIME"
