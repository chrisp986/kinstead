# Roadmap

## Current objective

Build a testable Vertical Slice proving **labor + supply +
relationships** under asynchronous time.

## Foundation correction --- complete

-   pgx upgraded to the current supported v5 line
-   production historical calendar separated from the synthetic v0.3 calendar
-   character birth dates normalized and ages derived from the authoritative
    364-day game calendar
-   v0.3 seed, strategies, events, and 48-tick calendar isolated under the
    balancing scenario package
-   frozen v0.3 design reference preserved; current Go-runner divergence is
    explicitly documented and independently regression-tested
-   market travel ticks and transport cost derived from directed geography
-   narrow application ports added for stable persistence workflows
-   existing world/market/shipment SQL migrated toward generated `sqlc` queries
-   CI added for backend, PostgreSQL integration, frontend, and Playwright

The completed Chronicle, Farm Report, and SvelteKit milestones now feed the
calendar/time-model hardening work below.

## Implementation order

### 1. Core simulation --- established

-   world clock/tick engine
-   household/characters/resources
-   assignments
-   seasonal production
-   consumption
-   fatigue
-   PostgreSQL transactional tick path

### 2. Shipments --- complete

Implement first-class shipments in the real transactional tick path: -
domain lifecycle - persistence/sqlc - due arrival processing as tick
step 1 - exactly-once inventory credit - cancellation - tests for
timing, idempotency, rollback - minimal read API/dev scenario

Implemented: domain lifecycle, transactional persistence, due arrivals at tick
step 1, exactly-once credit, timing/idempotency/rollback tests, read API,
development scenario, and sender-authorized direct-shipment cancellation with
an exactly-once stock refund. Market-created shipments deliberately require a
future explicit reversal workflow and cannot use direct cancellation.

### 3. Market --- established

-   offer queries
-   atomic purchase command
-   distance/transport cost
-   purchase creates shipment
-   concurrency tests
-   market API

Implemented, including concurrent purchase coverage and geography-derived
travel/transport values.

### 4. Contracts + relationships --- complete

-   recurring obligations
-   fulfillment by arrival
-   late/broken states
-   trust/history events
-   contract API

Complete: recurring obligations, atomic proposal persistence, counterparty
acceptance/rejection, arrival-based fulfillment, late/broken outcomes,
shipment-linked dispatch, deterministic contract rollups, relationship history,
explicit directional trust deltas, exactly-once trust application with clamps,
REST/OpenAPI projections, generated frontend types, and the household
contract/relationship UI. Calendar-scheduled trust outcomes are +2 on
time/early, -1 at 1--7 days late, -2 at 8--14 days late, and -8 when broken
at 15+ days late. Legacy tick-backed contracts retain the frozen v0.3 buckets;
the creditor's trust in the debtor changes and no reverse relationship is
modified automatically.

### 5. Politics --- complete

-   Jarl demands
-   decision expiry/defaults
-   labor/resource opportunity costs
-   relationship effects

Complete: deterministic Jarl labor-service and levy demands, expiry/default
refusal, transactional resource and assignment consequences, score clamps and
derived standing, structured chronicle outcomes, REST/OpenAPI projections,
generated client, and household Politics UI.

### 6. Chronicle + Farm Report --- complete

Implemented structured household Chronicle facts, idempotent contract-outcome
references, deterministic recent-change selection, and a prioritized Farm
Report with "what changed / needs attention / decide now" sections. The
report uses narrow read-model ports and structured item codes rather than
backend-localized prose. Minimal post-tick emergency food protection schedules
one normal next-tick assignment for an otherwise-free full-capacity worker and
records the action; future emergency work can be replaced transactionally by
the player's plan.

### 7. SvelteKit Vertical Slice --- complete

Complete: nested household routes for Farm Report, Farm, Work Planning, Trade,
and Chronicle; shared household shell with responsive navigation; mobile-first
decision cards and route-linked actions; route-owned data loading/forms;
responsive work, politics, market, contract, transit, relationship, and
chronicle surfaces; accessible feedback and mobile workflow coverage.

Generate API client/types from OpenAPI.

The final Vertical Slice UI organization is now the next product-polish focus.

### 8. Game-Time Model Migration --- complete

The authoritative `game_day` transition is complete for the current vertical
slice: production seasons and character ages use the deterministic 364-day
calendar; contract deadlines use game days with server-derived execution ticks;
shipment and Chronicle projections persist game-day snapshots; the calendar
read model covers seasonal boundaries, obligations, shipment arrivals, farm
markers, and political deadlines; and the setting-aware UI renders historical
years and relative calendar language. Goose upgrade/fresh equivalence coverage,
tick-boundary tests, contract day-bucket tests, shipment projections, and the
existing Farm Report accelerated-world cap prove that wall-clock tick duration
does not change calendar results. Tick fields retained in the schema are
execution, compatibility, or diagnostic projections only.

## First end-to-end economic scenario

Björnvik is low on provisions → player buys nearby provisions → silver
is debited → shipment travels → ticks advance → shipment arrives → stock
increases → chronicle records it → Farm Report reflects improved supply.

## After the Vertical Slice

Only after the core loop is proven: deeper historical events,
succession/generations, richer transport, broader geography, and
probabilistic/Monte Carlo balancing.

Explicitly defer PvP/combat, raids, ships/escorts, clans/chat, large
maps, complex religion/marriage/inheritance, and resource sprawl.
