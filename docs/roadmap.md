# Roadmap

## Current objective

Build a testable Vertical Slice proving **labor + supply +
relationships** under asynchronous time.

## Foundation correction --- complete

-   pgx upgraded to the current supported v5 line
-   production historical calendar separated from the synthetic v0.3 calendar
-   character birth dates normalized and ages derived from historical dates
-   v0.3 seed, strategies, events, and 48-tick calendar isolated under the
    balancing scenario package
-   frozen v0.3 design reference preserved; current Go-runner divergence is
    explicitly documented and independently regression-tested
-   market travel ticks and transport cost derived from directed geography
-   narrow application ports added for stable persistence workflows
-   existing world/market/shipment SQL migrated toward generated `sqlc` queries
-   CI added for backend, PostgreSQL integration, frontend, and Playwright

Next: begin contracts and relationships in the incremental order below.

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

### 4. Contracts + relationships --- next major milestone

-   recurring obligations
-   fulfillment by arrival
-   late/broken states
-   trust/history events
-   contract API

Progress: contract/obligation domain rules, atomic proposal persistence, and
counterparty acceptance/rejection with retry-safe recurring-obligation
generation are complete. Contract evaluation now runs at canonical tick step 2
after shipment arrivals, with arrival-based fulfilled, late, and broken
obligation states. Next: contract shipment linkage and lifecycle rollups.

### 5. Politics

-   Jarl demands
-   decision expiry/defaults
-   labor/resource opportunity costs
-   relationship effects

### 6. Chronicle + Farm Report

-   structured chronicle entries
-   "what changed / needs attention / decisions now"
-   emergency action reporting
-   concise notification rules

### 7. SvelteKit Vertical Slice

Screens: - Farm Report - Farm - Work Planning - Trade - Chronicle

Generate API client/types from OpenAPI.

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
