# ADR-0001: Absolute Game-Day Time Model

## Status

Accepted

## Date

2026-09-03

## Context

The game is set in Scandinavia around 980 CE, a period in which historical
timekeeping was not equivalent to a single modern civil calendar used uniformly
by everyone. At the same time, the simulation needs deterministic scheduling
for production, travel, recurring contracts, seasons, aging, events, and
absence processing.

Using modern month/day dates as authoritative domain state would couple the
simulation to a presentation convention and make historically different
calendar representations harder to introduce later. Using simulation ticks as
the calendar would create a different coupling: balance changes to tick cadence
would silently change the meaning of contracts, ages, seasons, and deadlines.

## Decision

The authoritative simulated calendar uses an absolute integer `game_day`.

- `game_day = 0` is the initial world snapshot.
- One gameplay year is 364 days / 52 weeks.
- One year contains four seasons.
- Each season is 91 days / 13 weeks.
- Year, season, week, and day labels are derived from `game_day`.
- The world's historical setting/year label is presentation/context metadata,
  not a SQL civil date.
- Simulation ticks are execution cadence and are separate from calendar time.
- The v0.3 `48 ticks/year, 12 ticks/season` model remains synthetic balancing
  data and does not define the production calendar.

The committed clock has an explicit interval rule. `current_tick` counts fully
committed execution steps and `current_game_day` is the calendar position
after them. A tick begins at the current game day, uses that day's production
season, and commits its effects at the next game day. Player actions between
ticks use the current day; tick-generated effects use the next day. With the
default `91 / 12` pacing this means ticks 1--12 use spring, tick 12 ends at
day 91, tick 13 uses summer, and tick 48 ends at day 364. Wall-clock tick
duration is only scheduling and is not an input to this model.

The gameplay calendar is a deliberate abstraction. It is not presented as an
exact reconstruction of a single medieval Swedish calendar.

## Persistence rules

Simulated dates and deadlines use integer game-day values, for example
`start_game_day`, `due_game_day`, and `next_delivery_game_day`. Political
response deadlines and shipment departure/arrival projections are also stored
as game-day snapshots.

SQL `DATE`, `TIME`, and `TIMESTAMP` types must not be used for simulated
historical dates or simulation deadlines. Real-world operational and audit
timestamps continue to use `TIMESTAMPTZ` where appropriate.

Derived values such as season name or week-of-season must not be persisted as
authoritative state unless a concrete projection/performance need is shown;
they must remain reproducible from the source clock.

## Contract scheduling

Recurring contracts schedule obligations against the authoritative game clock.
The initial implementation should support recurring intervals expressed in game
days. Each generated obligation receives an absolute game-day arrival deadline.
Calendar contracts classify lateness as 1--7 days, 8--14 days, and 15+ days;
legacy v0.3 tick-backed contracts retain their frozen tick outcomes. Political
demand service duration remains tick-based, while the response deadline is
snapshotted in game days when the demand is issued.

For calendar contracts, `due_game_day` is the business deadline and any
`due_arrival_tick` is a derived worker/execution projection. Arrival lateness
is 0 days (+2 trust), 1--7 days (-1), 8--14 days (-2), and 15+ days broken
(-8). Shipment and Chronicle game-day fields follow the same effective-day
rule: dispatch/player actions use the current day, while arrivals and other
tick consequences use the next day.

Season-bound and domain-event schedules may be added explicitly when useful,
for example `spring start` or a future market/assembly event. These rules must
resolve deterministically to game-day obligations.

Player-facing text should prefer phrases such as:

- `every two weeks`
- `next delivery in 9 days`
- `third week of winter`
- `ends at the beginning of spring`

Modern month/day date pickers are not the normal gameplay interface.

## Consequences

### Positive

- Calendar semantics remain stable when simulation tick pacing changes.
- Contract and event scheduling becomes deterministic and simple to compare.
- Seasons and ages can be derived consistently from one authoritative clock.
- Historical/calendar presentation can evolve without migrating core domain
  state.
- Tests and simulator runs can use small integer time values rather than civil
  date libraries.

### Costs

- The application needs explicit conversion/projection logic between ticks,
  game days, and player-facing calendar labels.
- Tick-based mechanics such as current travel balance values must resolve their
  expected arrival against the authoritative game clock server-side.
- Historical UI labels require a presentation layer instead of relying on
  standard date formatting libraries.

## Rejected alternatives

### Julian/Gregorian civil dates as source of truth

Rejected because they impose a modern/civil representation on core simulation
state and make alternate historical presentation unnecessarily expensive.

### Simulation ticks as calendar dates

Rejected because tick cadence is a balancing/execution concern. Changing tick
frequency must not redefine seasons, ages, contract intervals, or deadlines.

### Persisting year/season/week independently

Rejected because it creates multiple clocks that can disagree. These values are
projections of `game_day`.
