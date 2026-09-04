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

Politics uses a dedicated narrow port for Jarl-demand projections, responses,
and tick generation/expiry. Demand terms are snapshotted in the decision;
responses lock the world and decision in a serializable transaction, apply
resource or assignment consequences, update the political score, and record a
structured chronicle fact together.

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

The committed world clock has two distinct coordinates: `current_tick` is the
number of fully committed execution steps, while `current_game_day` is the
calendar position after those steps. Processing tick `current_tick + 1` starts
at the current game day and ends at the next game day. Production uses the
season at the start day; calendar effects that become true during the tick
use the end day. Player actions submitted between ticks use the world's
current game day.

Ticks are execution cadence, not calendar units. The production clock
advances simulated time deterministically after each processed tick according
to the world's pacing policy. The synthetic v0.3 `48 ticks/year,
12 ticks/season` values remain balancing inputs and do not define production
calendar semantics.

The default production pacing is `91 / 12` game days per tick: ticks 1--12
produce in spring, ticks 13--24 in summer, ticks 25--36 in autumn, and ticks
37--48 in winter. After tick 48 the committed clock is day 364, the start of
year 2 and spring. `tick_duration_seconds` is only the worker's wall-clock
wait between execution attempts and cannot change these results.

Use PostgreSQL row locking/`FOR UPDATE SKIP LOCKED` for worker
coordination. Avoid long-lived in-memory authoritative state.

## Game time

Authoritative simulated historical time is an absolute integer `game_day`.
`game_day = 0` is the world's initial snapshot. A gameplay year contains
**364 days / 52 weeks**, divided into **four 91-day / 13-week seasons**.
Derived year, season, week, and day labels are projections over `game_day` and
must never become independent authoritative state.

A world may carry a display anchor for historical context, such as the
starting year label `980 CE`, but simulated time is not stored as a Julian,
Gregorian, or SQL civil date. The gameplay calendar is intentionally a stable
simulation abstraction so historical presentation can evolve without
migrating domain state.

Persist simulated temporal values as integer game-day values, for example:

- `game_day`
- `start_game_day`
- `end_game_day`
- `due_game_day`
- `next_delivery_game_day`
- `available_from_game_day` and `expires_game_day` for issued political demands
- `departure_game_day`, `expected_arrival_game_day`, and
  `actual_arrival_game_day` for shipment calendar snapshots

Do not use SQL `DATE`, `TIME`, or `TIMESTAMP` for simulated dates or
simulation deadlines. Real-world operational/audit fields such as
`created_at`, `updated_at`, lease timestamps, and observability timestamps use
`TIMESTAMPTZ` as appropriate.

Calendar formatting belongs to domain/application presentation logic, not SQL.
Normal UI should derive phrases such as `third week of winter`, `in 5 days`,
or `every two weeks` from authoritative game time. Exact `game_day` values may
be exposed for diagnostics.

## Data model

Core entities:
`World, Location, Household, Character, CharacterSkill, ResourceStock, Assignment, Building, MarketOffer, Shipment, Contract, ContractObligation, Relationship, RelationshipEvent, WorldEvent, HouseholdDecision, PoliticalActor, PoliticalRelationship, ChronicleEntry, ProcessedWorldTick, OutboxEvent`

Use normalized tables for stable concepts. JSONB is for flexible
event/chronicle metadata.

Use fixed-point `BIGINT` for resources/money (`1000 = 1 unit`). Store
birth game day (or equivalent age anchor) and derive age from the authoritative
game clock.

Contract schedules persist rules, not rendered calendar labels. The vertical
slice should use absolute game-day deadlines plus recurring day intervals.
Season-triggered or other domain-event schedules can be represented explicitly
when added; they must resolve deterministically to authoritative game-day
obligations.

## API

REST endpoints represent commands and projections, not raw DB CRUD.
Examples: assign character, buy offer, propose contract, respond to
political demand.

OpenAPI is the Go↔SvelteKit contract. Generate TypeScript client/types
from it.

API projections may expose derived calendar labels and relative durations,
but commands should submit domain intent or validated schedule rules rather
than client-computed civil dates.

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

Recurring contract generation uses the authoritative game clock. A generated
obligation receives a game-day arrival deadline. Shipment execution may still
use tick-based travel balance values; expected arrival and lateness are resolved
server-side from authoritative tick/game-clock state rather than client date
arithmetic.

Calendar contract deadlines are absolute `due_game_day` values. Any
`due_arrival_tick` retained in storage is a derived execution projection, not
a player-authoritative deadline. Calendar lateness is 0 days on time (+2
trust), 1--7 days (-1), 8--14 days (-2), and 15 or more days broken (-8).

For temporal writes, player actions between ticks record the current game day;
tick-generated effects record the next game day. This applies to shipment
departure versus arrival, contract assessment, assignment consequences, and
Chronicle entries. PostgreSQL stores these values and validates them; calendar
rules and tick/game-day conversion live in `internal/calendar` and the
application/domain layers.

Contract dispatch reserves the exact obligated goods, derives travel time and
transport cost from directed geography, creates the physical shipment, and
links it to the obligation in one transaction.

Calendar-based contract obligations use immutable lateness buckets: arrival on
or before the due game day is fulfilled; 1--7 days late is the first late
bucket (-1 trust); 8--14 days late is the second late bucket (-2 trust); and
15 or more days late is broken (-8 trust). Legacy tick-backed contracts retain
the frozen v0.3 tick buckets until they are explicitly replaced by a calendar
schedule.

Final contract-obligation outcomes append directed relationship events in the
same tick transaction. Calendar-scheduled obligations use the day buckets
above; the event source is the creditor and the target is the debtor. The
legacy tick path continues to apply the frozen v0.3 outcomes: fulfilled on time
or early +2, one tick late -1, two ticks late -2, and broken/3+ ticks -8. The
current relationship row is an exactly-once projection of those event deltas,
clamped to -100..100. Temporary overdue states produce no event, and each
obligation produces at most one final consequence.

The calendar projection is assembled from authoritative sources rather than
stored as a duplicate event table. `GET /api/households/{id}/calendar`
range-reads contract obligations, dispatch deadlines, shipment arrivals,
political deadlines, assignments, seasonal boundaries, and recurring anchors
such as harvest, Jól, and Þing. The default range is the current day through
the next 182 days. Political response deadlines snapshot their game day when a
demand is issued; their service duration remains a tick-based balancing
mechanic.

Calendar ranges distinguish omitted parameters from an explicit day zero,
accept at most 364 days, and reject unknown event categories. Events are
structured facts (season, shipment, contract, politics, world, or farm) so
the frontend can render setting-appropriate language without duplicating
calendar arithmetic.

Prefer DB transactions/constraints to application mutexes.

## Infrastructure policy

Do not add Redis, Kafka, Kubernetes, microservices, GraphQL, CQRS, event
sourcing, or WebSockets without a concrete demonstrated need.
