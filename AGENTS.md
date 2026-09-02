# AGENTS.md

## Project

Async historical dynasty/economy strategy game (~980 CE).

Core: **labor, supply, relationships**. Players manage named household members, production, trade, shipments, contracts, politics, and chronicle.

`Kinstead` is a provisional display name. Never use it in packages, DB tables, APIs, IDs, or infrastructure naming.

## Stack

* Go backend
* SvelteKit + TypeScript frontend
* PostgreSQL
* `pgx` + `sqlc`
* REST + OpenAPI
* Docker
* Go tests + Playwright

Modular monolith. Separate API, worker, simulator binaries; shared Go domain/simulation code.

Avoid microservices, Redis, Kafka, GraphQL, CQRS, event sourcing, Kubernetes unless justified.

## Architecture

```text id="82r4xu"
HTTP/Worker → Application → Domain
                  ↓           ↑
                Ports     Simulation
                  ↑
              PostgreSQL
```

Rules:

* PostgreSQL = authoritative state.
* Domain/simulation must not depend on HTTP, DB, `pgx`, `sqlc`, JSON, or frontend.
* Client sends intent; server computes outcomes.
* Production and `cmd/simulator` use identical simulation rules.

## Simulation

Deterministic tick order:

```text id="gkq7vo"
shipments → contracts → assignments → production → consumption
→ fatigue/health → events → emergency AI → chronicle/metrics
```

Each tick is sequential, atomic, and idempotent.

Idempotency key:

```text id="w8swwu"
(world_id, tick)
```

Use PostgreSQL transactions and `FOR UPDATE SKIP LOCKED` for worker coordination. Process missed ticks sequentially.

### Time

Balancing only:

```text id="m7j7hg"
48 ticks = abstract year
12 ticks = abstract season
```

These are **not historical days**. Keep real time, simulation ticks, and historical date separate/configurable.

## Numeric Rules

No floats for authoritative resources/money.

```text id="gprv45"
1000 = 1 unit
```

Prefer distinct Go types for IDs, ticks, quantities, money.

## Gameplay

Characters are people, not worker points. Assign work to specific characters.

Work durations:

```text id="5ulc38"
1 / 3 / 6 / 12 ticks
```

Production completes automatically. **Never add collect buttons.**

Supply:

```text id="gwjshw"
>30 safe
15–30 strained
<15 critical
<7 emergency
```

Absence must not destroy a household. Emergency AI makes conservative, explainable decisions.

More clicks must not proportionally increase production.

## Balance

v0.3 is frozen unless intentionally rebalancing.

```text id="jtd1o1"
start: 150 provisions, 20 wood, 4 trade goods, 30 silver
consumption: 4.9 provisions/tick
wood upkeep: 1.0/tick
woodcutting: 3.0/worker/tick
```

Gameplay parameters belong in versioned Go config/domain code, not DB rows by default.

## Trade

Goods never teleport. Purchases create shipments.

Market purchase transaction:

```text id="jtk3mi"
lock offer → validate → debit/reserve → update offer
→ create shipment → commit
```

Never trust client-computed prices, balances, quantities, or travel times.

Contracts generate `ContractObligation`s. Fulfillment depends on **arrival**, not dispatch.

Relationships store both current state and history (fulfilled/late/broken obligations, trade, political actions).

## Chronicle

Store structured facts, not only rendered prose. Render text from facts for localization/UI.

## API / DB

REST + OpenAPI. Expose commands/projections, not raw CRUD.

Good:

```text id="eox4fm"
assign character
buy offer
propose contract
respond to demand
```

Bad:

```text id="omfukn"
PATCH /resource_stocks/{id}
```

Generate frontend types/client from OpenAPI.

Use migrations and DB constraints for invariants. Use JSONB only for flexible metadata/events, not stable domain models.

Prefer DB transactions/locking over application mutexes.

## Tests

Simulation changes require tests for relevant behavior:

* deterministic/idempotent ticks
* production/consumption
* fatigue/modifiers
* emergency AI
* shipments
* atomic market trades
* contracts/lateness

Do not change v0.3 regression expectations just to pass tests.

## Scope

Priority:

```text id="v6c9z7"
core simulation → shipments → market → contracts/relationships
→ politics → chronicle/report → SvelteKit UI
```

Do not add PvP, combat, raids, ships, complex inheritance/religion, large maps, clans, chat, or resource sprawl without explicit scope change.

## Engineering

* Prefer simple, explicit code.
* Avoid premature abstractions/infrastructure.
* Interfaces live where consumed.
* Treat client input as untrusted.
* Use structured errors/logging.
* Keep game rules out of HTTP handlers and SQL.

Before adding an interaction ask:

> What meaningful decision does this create?

No decision → probably no interaction.

**Invariant:** PostgreSQL stores truth; domain defines rules; simulation advances the world; worker executes time; API accepts player decisions.
