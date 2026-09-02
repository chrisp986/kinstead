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

Shipment arrival credits the destination exactly once inside tick
processing.

Contract fulfillment is determined by shipment arrival.

Prefer DB transactions/constraints to application mutexes.

## Infrastructure policy

Do not add Redis, Kafka, Kubernetes, microservices, GraphQL, CQRS, event
sourcing, or WebSockets without a concrete demonstrated need.
