# AGENTS.md

## Project

Async historical dynasty/economy strategy game (~980 CE). Core:
**labor, supply, relationships**. `Kinstead` is provisional branding;
never use it in packages, DB/API names, IDs, or infrastructure.

## Stack

Go; SvelteKit + TypeScript; PostgreSQL 18; `pgx` + `sqlc`; REST +
OpenAPI; Docker; Go tests/Vitest/Playwright. Modular monolith with
shared simulation code for API, worker, and simulator. Avoid extra
infrastructure unless justified.

## Core architecture

-   PostgreSQL is authoritative.
-   Client sends intent; server computes outcomes.
-   Domain/simulation are infrastructure-independent.
-   Production and simulator share gameplay rules.
-   Use transactions/DB constraints for concurrency and invariants.
-   No floats for authoritative economy values: `1000 = 1 unit`.
-   Keep game rules out of HTTP handlers and SQL.

Tick order:
`shipments → contracts → assignments → production → consumption → fatigue/health → events → emergency AI → chronicle/metrics`

Ticks are sequential, atomic, idempotent by `(world_id, tick)`. Process
missed ticks sequentially.

Authoritative simulated time is absolute integer `game_day`. A gameplay
year is 364 days = 52 weeks = four 13-week seasons. Derive calendar
labels from `game_day`; never use SQL `DATE`/`TIMESTAMP` for simulated
dates. Real-world audit timestamps use `TIMESTAMPTZ`. Ticks are execution
cadence, not calendar units. The v0.3 `48 ticks/year`, `12/season` model
is balancing data, not the production calendar.

## Gameplay constraints

Characters are named people; work targets characters. Production
completes automatically: **no collect buttons**. Absence must be safe;
emergency AI is conservative and explainable. More clicks must not
proportionally increase output.

v0.3 balance is frozen unless explicitly rebalancing. Goods never
teleport: trade creates shipments; contract fulfillment depends on
arrival.

Relationships store history, not only scores. Chronicle stores
structured facts.

## API / DB

REST + OpenAPI; expose commands/projections, not raw CRUD. Generate
frontend API types from OpenAPI. Use migrations. Normalize stable domain
concepts; reserve JSONB for flexible metadata/events.

## Tests

Simulation changes require relevant deterministic tests. Do not change
v0.3 regression expectations merely to pass tests.

## Scope / priority

`core simulation → shipments → market → contracts/relationships → politics → chronicle/report → SvelteKit UI`

No PvP, raids/combat, ships, complex inheritance/religion, large maps,
clans/chat, or resource sprawl without explicit scope change.

## Engineering

Prefer simple explicit code and small milestones. Treat client input as
untrusted. Use structured errors/logging.

Before adding an interaction ask: **What meaningful decision does this
create?**

Invariant: **PostgreSQL stores truth; domain defines rules; simulation
advances the world; worker executes time; API accepts player
decisions.**
