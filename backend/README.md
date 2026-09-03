# Backend milestone 5 — atomic market purchases

This milestone wires the deterministic Go simulation core into PostgreSQL and exposes the first endpoints needed by a future SvelteKit UI.

## What is implemented

- Atomic **one-world / one-tick** transaction.
- Due-world claiming via `FOR UPDATE SKIP LOCKED`.
- Serializable transactions and `(world_id, tick)` idempotency records.
- Persistent provisions, wood, trade goods, silver and character fatigue.
- Persistent work plans with 1 / 3 / 6 / 12 tick durations.
- Work-plan writes serialize against the same world row as tick processing.
- Assignment lifecycle: `planned -> active -> completed`.
- First-class shipments that reserve sender goods and arrive before household simulation.
- Idempotent, atomic arrival credits with structured `shipment_arrived` chronicle facts.
- Atomic market purchases with authoritative prices, stock checks, silver transfer,
  partial/full offer fills, and shipment creation.
- Household chronicle projection for purchases, sales, arrivals, and assignment lifecycle facts.
- Farm-report read model with supply days, resources, characters, plans and up to three alerts.
- REST endpoints:
  - `GET /healthz`
  - `GET /api/households/{id}/report`
  - `GET /api/households/{id}/assignments`
  - `POST /api/households/{id}/assignments`
  - `GET /api/households/{id}/shipments`
  - `GET /api/households/{id}/chronicle`
  - `GET /api/market/offers?world_id={id}`
  - `POST /api/market/offers/{id}/purchase`
- PostgreSQL 18.6 Docker Compose setup.
- Bjornvik development seed.
- A 30-provision Hrafnstead-to-Bjornvik shipment due at tick 2.
- A Hrafnstead provision offer priced at 1.5 silver per unit.
- Offline Go simulation tests remain runnable without PostgreSQL dependencies.

## Production clock semantics

The v0.3 48-tick balancing year is **not** the production season calendar.
Production uses four distinct concepts: wall-clock scheduling, sequential
simulation ticks, a rational tick-to-historical-day conversion, and historical
dates/seasons.

- Tick 0 is the historical start-date snapshot.
- By default, 48 production ticks advance 365 historical days.
- Production season comes from the resulting historical month, not `tick % 48`.
- The isolated v0.3 simulator still uses 12 balancing ticks per synthetic season.
- Target pace: one game year in about 8 real days.
- Therefore the development seed uses one tick every **4 real hours** (`14400` seconds).

The database stores the conversion as `historical_days_per_tick_num /
historical_days_per_tick_den` (default `365 / 48`). Characters store a
historical `birth_date`; age is derived for the report date rather than stored
or inferred from balancing ticks.

## Requirements for full local test

- Go 1.27+
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

With a migrated local database running, execute the PostgreSQL arrival, idempotency,
and rollback tests instead of skipping them:

```bash
make test-integration
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

The smoke test reads the farm report, purchases part of the seeded offer, verifies that
the resulting shipment remains in transit, schedules Astrid, and forces one worker tick.

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

## Observe the seeded shipment

After `make db-reset`, start the API and worker, then inspect Bjornvik's inbound shipment:

```bash
curl http://localhost:8080/api/households/00000000-0000-0000-0000-000000000020/shipments
```

Force two sequential ticks (run the update once, wait for the worker, then repeat):

```bash
docker compose -f ../docker-compose.yml exec -T postgres \
  psql -U game -d game -c \
  "UPDATE worlds SET next_tick_at=now() WHERE id='00000000-0000-0000-0000-000000000001';"
```

The shipment remains `in_transit` after tick 1. At tick 2 it becomes `arrived`, stores
`actual_arrival_tick: 2`, and credits 30,000 milli-provisions before that tick's
production and consumption.

## Purchase the seeded market offer

List active offers, then buy five provisions for Bjornvik. The server calculates the
8,500 milli-silver total cost (7,500 goods + 1,000 local transport) and
two-tick arrival; the request supplies neither value. Both delivery time and
transport cost are derived from the directed route between the locations.

```bash
curl "http://localhost:8080/api/market/offers?world_id=00000000-0000-0000-0000-000000000001"

curl -X POST -H 'Content-Type: application/json' \
  http://localhost:8080/api/market/offers/00000000-0000-0000-0000-000000000302/purchase \
  -d '{
    "buyer_household_id":"00000000-0000-0000-0000-000000000020",
    "quantity_milli":5000
  }'
```

The purchase immediately transfers silver and reserves the seller's provisions, but
Bjornvik receives no provisions until the resulting shipment reaches its arrival tick.

## Deliberately not yet persisted

The online worker currently persists market-created shipment arrivals plus the core loop already implemented by `simulation.ProcessTick`: assignments, seasonal production, household consumption, wood upkeep and fatigue. The offline v0.3 scenario still contains richer strategy behavior such as automated trade, construction progression and Jarl decisions.

The next integration increment should add **contracts and obligations fulfilled by shipment arrival**, without duplicating rules in HTTP handlers.
