#!/usr/bin/env bash
set -euo pipefail

API="${API:-http://localhost:8080}"
HOUSEHOLD="00000000-0000-0000-0000-000000000020"
ASTRID="00000000-0000-0000-0000-000000000102"
WORLD="00000000-0000-0000-0000-000000000001"
OFFER="00000000-0000-0000-0000-000000000302"

echo "1) Health"
curl --fail --silent --show-error "$API/healthz"; echo

echo "2) Initial farm report"
curl --fail --silent --show-error "$API/api/households/$HOUSEHOLD/report"; echo

echo "3) List active market offers"
curl --fail --silent --show-error "$API/api/market/offers?world_id=$WORLD"; echo

echo "4) Purchase 5 provisions; delivery must remain in transit"
curl --fail --silent --show-error -X POST \
  -H 'Content-Type: application/json' \
  "$API/api/market/offers/$OFFER/purchase" \
  -d "{\"buyer_household_id\":\"$HOUSEHOLD\",\"quantity_milli\":5000}"; echo
curl --fail --silent --show-error "$API/api/households/$HOUSEHOLD/shipments"; echo

echo "5) Schedule Astrid to fish for 3 ticks"
curl --fail --silent --show-error -X POST \
  -H 'Content-Type: application/json' \
  "$API/api/households/$HOUSEHOLD/assignments" \
  -d "{\"character_id\":\"$ASTRID\",\"activity\":\"fishing\",\"intensity\":\"normal\",\"duration_ticks\":3}"; echo

echo "6) Force the next tick due now (development only)"
docker compose -f ../docker-compose.yml exec -T postgres \
  psql -U game -d game -v ON_ERROR_STOP=1 \
  -c "UPDATE worlds SET next_tick_at=now() WHERE id='00000000-0000-0000-0000-000000000001';"

echo "Waiting for worker..."
sleep 2

echo "7) Farm report after worker tick"
curl --fail --silent --show-error "$API/api/households/$HOUSEHOLD/report"; echo

echo "Smoke test completed. Verify that tick increased, Astrid changed, and the purchased shipment did not deliver before tick 2."
