# ADR-0002: Monthly Seasons and Daylight Work

## Status

Accepted

## Date

2026-09-11

## Context

The accelerated `daily_labor_v1` clock made a game year pass in roughly eight
real days and gave every date the same `[08:00,17:00)` work window. That model
did not give players an intuitive relationship between returning tomorrow,
seasonal change, and the household's routine.

The game still needs deterministic outcomes during outages and forecasts. A
player's device timezone and daylight-saving rules must never alter production.
The modern scheduling calendar also must not be mistaken for the household's
historical year or used to accelerate character aging.

## Decision

New worlds explicitly created with `monthly_seasons_v1` use:

- one 3,600-second real interval per simulation tick;
- one game hour per tick and 24 ticks per game day;
- a persisted `calendar_anchor_at` representing game day 0, hour 0 at midnight
  in the world's persisted fixed UTC offset;
- one scheduling-calendar month per season, with January/May/September as
  spring, February/June/October as summer, March/July/November as autumn, and
  April/August/December as winter; and
- actual month lengths, including leap-year February.

The fixed offset deliberately avoids daylight-saving days shorter or longer
than 24 hours. Interfaces may show a player's local conversion, but simulation
outcomes use only world time. Modern dates are an implementation schedule, not
the displayed historical year. Character aging keeps the model's historical
365-day definition and is not accelerated when the four-season cycle repeats.

Worker polling is independent of game speed. After each successful atomic
tick, `next_tick_at` is calculated from the preceding scheduled due time, not
from the wall clock. Missed intervals therefore execute sequentially with their
own anchored dates.

## Daylight and work boundaries

Versioned daylight anchors are gameplay approximations:

| Season start | Sunrise | Sunset |
| --- | ---: | ---: |
| Spring | 08:00 | 18:00 |
| Summer | 05:00 | 21:00 |
| Autumn | 06:00 | 18:00 |
| Winter | 09:00 | 15:00 |

Sunrise and sunset interpolate toward the next season's anchor over the actual
month length. Outdoor work starts one whole hour after sunrise and ends after
at most eight normal-effort hours or at sunset. The working interval is
`[start_hour,end_hour)`: an interval ending at `end_hour` performs settlement,
not another hour of production.

Daily rates are divided into eight integer hourly shares. Remainders are kept
in pending milli-units instead of being rounded with floating point. Short
winter days earn fewer shares rather than compressing a full day's reward into
fewer hours. At workday end, all valid pending output is deposited before that
interval's consumption. Deposit, consumption, pending reset, report facts, and
clock advancement share one PostgreSQL transaction, so a rollback removes all
effects and a retry cannot deposit twice.

Occupations are persistent. Temporary commitments replace effective activity
for their bounded interval without erasing the occupation. A change requested
before today's work starts activates at today's start; a request at or after a
work-start boundary already represented by committed state activates at
tomorrow's independently calculated start. Choosing the current occupation
cancels a pending change.

## Compatibility and conversion

The migration adds nullable anchor/offset columns and the new model identifier;
it does not update an existing world's model or clock. `legacy` and
`daily_labor_v1` dispatch through their frozen rules. Unknown model identifiers
are errors.

Converting an existing world requires a separate reviewed operation that picks
a world-time offset and midnight anchor consistent with its committed game
moment, validates pending occupation boundaries and duties, preserves stocks,
ages, contracts, shipments, and history, and schedules the first hourly due
interval without replaying or skipping time. No such automatic conversion is
provided by this ADR.

## Forecasts

Forecasts copy mutable household state and advance the same authoritative
clock one hour at a time. Every interval recalculates scheduling date, season,
and daylight; confirmed arrivals and supported commitments are applied at their
execution boundaries. Responses identify the snapshot and horizon and list
assumptions. A forecast is informative, not a reservation, and stale revisions
remain invalid for occupation commands.
