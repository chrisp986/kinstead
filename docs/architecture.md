# Architecture

## Stack

-   Frontend: SvelteKit + TypeScript
-   Backend: Go
-   API: REST + OpenAPI
-   Database: PostgreSQL 18
-   DB: `pgx` + `sqlc`
-   Migrations: Goose-compatible
-   Deployment: Docker
-   Tests: Go testing, Vitest, Playwright

Use a modular monolith. Separate `cmd/api`, `cmd/worker`, and
`cmd/simulator` binaries share the same domain/simulation packages.

## Boundaries

Conceptual flow:
`HTTP/Worker → Application → Domain/Simulation → Ports → PostgreSQL`

Domain/simulation must not depend on HTTP, PostgreSQL, pgx/sqlc, JSON,
or frontend code. PostgreSQL is authoritative; clients submit intent
only.

Application services depend on narrow ports for report reads, shipment
storage, market-purchase transactions, contract proposal/response/dispatch
transactions, chronicle reads, and world-tick transactions. PostgreSQL adapters
implement those ports; transaction ordering and game decisions remain in the
application/domain layers. Stable locking and
projection queries are generated with `sqlc`.

Production and balancing simulator must share the same gameplay
mechanics.

## Tick processing

Canonical order: 1. shipment arrivals 2. contract obligations 3.
assignments 4. production 5. consumption 6. fatigue/health 7. events 8.
emergency automation 9. chronicle/metrics

Each world tick is sequential and transactional:
`BEGIN → lock/load world → idempotency check → load state → process → persist → mark processed → advance clock → COMMIT`

Idempotency key: `(world_id, tick)`. A retry must not duplicate effects.
Process missed ticks one-by-one.

Tick 0 is the historical start-date snapshot. Processing tick N advances the
world through the interval ending at
`floor(N × historical_days_per_tick_num / historical_days_per_tick_den)` days
after that date. Production season is derived from the resulting historical
date (March--May spring, June--August summer, September--November autumn,
December--February winter). Wall-clock scheduling, production ticks,
historical dates, and the synthetic v0.3 48-tick calendar are separate.

Use PostgreSQL row locking/`FOR UPDATE SKIP LOCKED` for worker
coordination. Avoid long-lived in-memory authoritative state.

## Data model

Core entities:
`World, Location, Household, Character, CharacterSkill, ResourceStock, Assignment, Building, MarketOffer, Shipment, Contract, ContractObligation, Relationship, RelationshipEvent, WorldEvent, HouseholdDecision, PoliticalActor, PoliticalRelationship, ChronicleEntry, ProcessedWorldTick, OutboxEvent`

Use normalized tables for stable concepts. JSONB is for flexible
event/chronicle metadata.

Use fixed-point `BIGINT` for resources/money (`1000 = 1 unit`). Store
birth tick/date and derive age.

## API

REST endpoints represent commands and projections, not raw DB CRUD.
Examples: assign character, buy offer, propose contract, respond to
political demand.

OpenAPI is the Go↔SvelteKit contract. Generate TypeScript client/types
from it.

## Concurrency

Market purchases and transfers are atomic. Lock the offer, validate
quantity/funds, update balances/offer, create shipment, then commit.

Directed location routes are authoritative geography for the vertical slice.
Their distance class determines travel ticks and the server-calculated fixed
transport charge; clients cannot submit either value.
Long-distance travel is outside the vertical slice and remains unavailable for
market purchases until its transport price is explicitly designed.

Shipment arrival credits the destination exactly once inside tick
processing.

Direct shipments may be cancelled by their sender before the due tick. The
same transaction marks them cancelled, refunds the reserved goods once, and
records a chronicle fact; retries are idempotent. Market-created shipments are
excluded because reversing a completed sale requires a separate explicit
market workflow.

Contract fulfillment is determined by shipment arrival.

Contract dispatch reserves the exact obligated goods, derives travel time and
transport cost from directed geography, creates the physical shipment, and
links it to the obligation in one transaction.

Final contract-obligation outcomes append directed relationship events in the
same tick transaction. Fulfilled obligations on time or early apply +2 trust;
one tick late applies -1; two ticks late applies -2; and broken or 3+ ticks
late applies -8. The event source is the creditor and the target is the debtor.
The current relationship row is an exactly-once projection of those event
deltas, clamped to -100..100. Temporary overdue states produce no event, and
each obligation produces at most one final consequence.

Prefer DB transactions/constraints to application mutexes.

## Infrastructure policy

Do not add Redis, Kafka, Kubernetes, microservices, GraphQL, CQRS, event
sourcing, or WebSockets without a concrete demonstrated need.
