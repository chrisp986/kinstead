#!/usr/bin/env bash
set -euo pipefail

docker compose -f ../docker-compose.yml down -v
docker compose -f ../docker-compose.yml up -d postgres

echo "Waiting for PostgreSQL..."
for _ in $(seq 1 30); do
  if docker compose -f ../docker-compose.yml exec -T postgres pg_isready -U game -d game >/dev/null 2>&1; then
    ./scripts/bootstrap_db.sh
    exit 0
  fi
  sleep 1
done

echo "PostgreSQL did not become ready" >&2
exit 1
