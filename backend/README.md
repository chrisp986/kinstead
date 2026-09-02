# Backend milestone 3 — PostgreSQL tick + first REST API

This milestone wires the deterministic Go simulation core into PostgreSQL and exposes the first endpoints needed by a future SvelteKit UI.

## What is implemented

- Atomic **one-world / one-tick** transaction.
- Due-world claiming via `FOR UPDATE SKIP LOCKED`.
- Serializable transactions and `(world_id, tick)` idempotency records.
- Persistent provisions, wood, trade goods, silver and character fatigue.
- Persistent work plans with 1 / 3 / 6 / 12 tick durations.
- Work-plan writes serialize against the same world row as tick processing.
- Assignment lifecycle: `planned -> active -> completed`.
- Farm-report read model with supply days, resources, characters, plans and up to three alerts.
- REST endpoints:
  - `GET /healthz`
  - `GET /api/households/{id}/report`
  - `GET /api/households/{id}/assignments`
  - `POST /api/households/{id}/assignments`
- PostgreSQL 16 Docker Compose setup.
- Bjornvik development seed.
- Offline Go simulation tests remain runnable without PostgreSQL dependencies.

## Important clock correction

The 48-day balancing year is **not** a literal 48-day historical calendar. It is an abstract simulation year:

- 48 simulation ticks = 365 historical days.
- 12 ticks = one balancing season.
- Target pace: one game year in about 8 real days.
- Therefore the development seed uses one tick every **4 real hours** (`14400` seconds).

The database stores the historical conversion as `historical_days_per_tick_num / historical_days_per_tick_den` (default `365 / 48`) instead of hard-coding it into UI code.

## Requirements for full local test

- Go 1.23+
- Docker + Docker Compose
- Internet access once, so Go can download `pgx`
- `curl`

You do **not** need local `psql` or `goose`; the bundled scripts run SQL through the PostgreSQL container.

## 1. Core tests

These do not require PostgreSQL or downloaded pgx code:

```bash
make test
make sim
```

## 2. Download DB dependency and compile full backend

```bash
make deps
make test-postgres
```

## 3. Create a fresh development database

```bash
make db-reset
```

This starts PostgreSQL, applies the `Up` parts of the migrations and inserts the Bjornvik seed.

## 4. Start API and worker

Terminal A:

```bash
make api
```

Terminal B:

```bash
make worker
```

## 5. Run the smoke test

Terminal C:

```bash
make smoke
```

The smoke test reads the farm report, creates a three-tick fishing assignment for Astrid, forces one development tick due, waits for the worker and reads the report again.

## Useful development IDs

- World: `00000000-0000-0000-0000-000000000001`
- Bjornvik: `00000000-0000-0000-0000-000000000020`
- Bjorn: `00000000-0000-0000-0000-000000000101`
- Astrid: `00000000-0000-0000-0000-000000000102`
- Einar: `00000000-0000-0000-0000-000000000103`
- Ragnhild: `00000000-0000-0000-0000-000000000104`
- Sven: `00000000-0000-0000-0000-000000000105`

## Example: manually create a work plan

```bash
curl -X POST \
  -H 'Content-Type: application/json' \
  http://localhost:8080/api/households/00000000-0000-0000-0000-000000000020/assignments \
  -d '{
    "character_id":"00000000-0000-0000-0000-000000000102",
    "activity":"fishing",
    "intensity":"normal",
    "duration_ticks":3
  }'
```

The API rejects overlapping work plans and plans that start in an already processed tick.

## Deliberately not yet persisted

The online worker currently persists the core loop already implemented by `simulation.ProcessTick`: assignments, seasonal production, household consumption, wood upkeep and fatigue. The offline v0.3 scenario still contains richer strategy behavior such as automated trade, construction progression and Jarl decisions.

The next integration increment should move **shipments + market**, then **contracts**, into the same transactional production path rather than duplicating those rules in HTTP handlers.
