# Roadmap

## Current objective

Build a testable Vertical Slice proving **labor + supply +
relationships** under asynchronous time.

## Implementation order

### 1. Core simulation --- established

-   world clock/tick engine
-   household/characters/resources
-   assignments
-   seasonal production
-   consumption
-   fatigue
-   PostgreSQL transactional tick path

### 2. Shipments --- next

Implement first-class shipments in the real transactional tick path: -
domain lifecycle - persistence/sqlc - due arrival processing as tick
step 1 - exactly-once inventory credit - cancellation - tests for
timing, idempotency, rollback - minimal read API/dev scenario

### 3. Market

-   offer queries
-   atomic purchase command
-   distance/transport cost
-   purchase creates shipment
-   concurrency tests
-   market API

### 4. Contracts + relationships

-   recurring obligations
-   fulfillment by arrival
-   late/broken states
-   trust/history events
-   contract API

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
