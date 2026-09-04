# Project Specification

## Product

An asynchronous historical dynasty strategy game beginning in
Scandinavia around **980 CE**. The player leads a farmstead and family
through economic, social, and political change.

Core fantasy: **You do not build a base. You lead a family and its
farmstead.**

Pillars: **Labor, Supply, Relationships**. Geography and transport
connect them.

## Design

Target ~5-minute sessions and 1--2 sessions/day. Several days of
absence must not destroy the household. Frequent activity may improve
planning/information, but must not multiply base output.

Primary rule: **Do not create clicks that contain no decision.**

Desired pressure: **I could have enough --- just not for everything at
once.**

Core loop:
`Farm report → assess → allocate labor → secure supply → trade/obligations → time passes → consequences → chronicle`

Vertical Slice question: **Is it fun to lead a farmstead with limited
labor through several seasons while building relationships with other
farmsteads?**

## Setting and time

Start in 980 CE. Birka represents the older trade order and may decline;
Sigtuna can rise. Christianization and stronger royal power are visible
but incomplete. Historical tendencies should create gameplay
opportunities rather than rigid scripting.

Target pacing: `1 game year ≈ 8 real days`; a generation ≈ 18--25 game
years.

The authoritative simulated calendar is based on an absolute integer
`game_day`. A gameplay year has **364 days / 52 weeks**, divided into
**four seasons of 13 weeks / 91 days**. Year, season, week, day labels,
and relative durations are derived from `game_day`; they are not
independent sources of truth.

Normal player-facing time should use seasons, weeks, days, relative
phrasing, and meaningful events rather than modern month/day notation.
Examples: `third week of winter`, `in 5 days`, `every two weeks`, or
`at the beginning of spring`. The 980 CE setting remains historical
context; the gameplay calendar is a deliberate simulation abstraction,
not a claim to reconstruct one exact medieval Swedish calendar.

Wall-clock scheduling, simulation ticks, and `game_day` are separate
concepts. Production pacing deterministically advances the world clock;
ticks are execution cadence and must not become calendar semantics. The
v0.3 balancing model's `48 ticks/year, 12/season` remains synthetic
balancing data and must not define the production calendar.

## Starting household

Reference farm: **Hof Björnvik**.

  Character     Age   Labor Specialization
  ----------- ----- ------- ----------------
  Björn          32     1.0 Agriculture
  Astrid         29     1.0 Fishing
  Einar          17     1.0 None
  Ragnhild       13     0.5 Training
  Sven            6       0 Child

Characters support birth/derived age, relationships, labor capacity,
skills, training, health, fatigue, and assignment.

## Resources and labor

Resources: provisions, wood, trade goods, silver. Authoritative
quantities are fixed-point integers: `1000 = 1 unit`.

Starting stock: 150 provisions, 20 wood, 4 trade goods, 30 silver.

Activities: agriculture, fishing, woodcutting, crafting, training,
market/travel, ruler service, rest. Work is assigned to specific
characters and completes automatically.

Intensity: - light: 80% production, +2 fatigue - normal: 100%, +4 -
high: 120%, +7 - rest: -12 fatigue

Fatigue: 0--49 normal; 50--69 warning; 70--84 -10% production; 85--100
-25% plus higher event risk.

## Supply

Communicate supply as "Provisions last X days." Thresholds: >30 safe;
15--30 strained; <15 critical; <7 emergency.

Emergency AI should pause nonessential construction, stop high-intensity
work, shift labor toward food, sell limited reserves, and buy NPC food
if needed. Preserve ~10 silver where possible. Log all automatic
decisions.

## Trade and geography

Households should not efficiently produce everything. Market handles
short-term exchange; contracts create recurring relationships.

Goods never teleport. Physical transfers create shipments. Reference
travel times: neighbor 1, local 2, near regional 3, regional 5, far
regional 8, long distance 12 balancing ticks. Most slice shipments:
1--5.

Shipment lifecycle: `prepared → in_transit → arrived`. Purchased
resources enter buyer stock only on arrival.

Market purchase must be atomic: lock offer → validate → debit/reserve →
update offer → create shipment → commit.

Contracts generate obligations. Fulfillment is based on **arrival**, not
dispatch. 1--2 ticks late = late; 3+ = broken/unfulfilled. Relationship
history is the main consequence, not a generic money penalty.

Recurring contract schedules use the game calendar rather than civil
calendar dates. The default recurring form is an interval in game days;
season-bound or domain-event triggers may be added where they create a
meaningful historical or gameplay distinction. Each generated obligation
has an arrival deadline tied to the authoritative game clock.

Examples of player-facing schedules:

- `20 sacks of barley every two weeks`
- `next delivery in 9 days`
- `deliver at the beginning of winter`
- `contract ends at spring start`

Modern date pickers and month/day deadlines are not part of normal
contract gameplay. Internal schedule state stores absolute game-day
values and schedule rules, not rendered labels.

Contract trust consequences for calendar-scheduled contracts:

- fulfilled on time or early: +2
- fulfilled 1--7 days late: -1
- fulfilled 8--14 days late: -2
- broken / 15+ days late: -8

Existing tick-backed v0.3 contracts retain their original one-/two-tick late
classification. New recurring contracts use 7, 14, or 28 game-day intervals;
the server resolves seasonal end conditions to absolute game days. Expected
shipment travel is still a tick-based simulation mechanic, but departure and
arrival game days are snapshotted for calendar and deadline projections.

A contract obligation affects the creditor's trust in the debtor. Each
obligation produces at most one final trust consequence. Trust is clamped to
-100..100. Contract trust currently does not scale by quantity, value,
resource type, distance, or contract duration.

## Relationships, politics, chronicle

Relationships combine a current score/state with readable event history:
fulfilled/late/broken obligations, repeated trade, political
cooperation/conflict.

Vertical Slice political NPC: local Jarl. Internal score -100..100;
player states: disapproving (-100..-31), neutral (-30..29), favorable
(30..69), connected (70..100). Political demands consume labor/resources
and create opportunity costs.

Contract trust consequences are fixed and directional. Calendar contracts use
the day buckets above; legacy tick-backed v0.3 contracts use fulfilled on time
or early +2, one tick late -1, two ticks late -2, and broken/3+ ticks -8. Each
obligation produces at most one consequence; trust is clamped to -100..100 and
does not scale by quantity, value, resource type, distance, or contract
duration.

Jarl demands use production event types `political_labor_service` and
`political_levy`. Labor service reserves one full-capacity character for four
ticks. Levy honor costs 18,000 wood or 6,000 silver milli-units. Honoring is
+10 political score; refusal or expiry default is -5. Political standing is
derived from score and is never persisted separately.

Chronicle stores structured facts and renders prose later. Record major
trades, contracts, buildings, political choices, shipment issues, and
emergency actions.

## UI

Primary screens: 1. Farm Report 2. Farm 3. Work Planning 4. Trade 5. Calendar
6. Chronicle

Farm Report answers: What changed? What needs attention? What meaningful
decisions matter now? Prefer 2--3 important actions.

Time-sensitive UI should prefer derived, readable game-calendar labels such
as season/week and relative durations. Exact internal `game_day` values may
be exposed in diagnostics, but are not the normal player-facing format.

The Calendar screen offers `Upcoming` (grouped by urgency and half-year) and
`Year cycle` (the summer/winter rhythm with seasonal phases and anchors). It
projects contract due dates, dispatch deadlines, shipment arrivals, political
response deadlines, farm markers, festivals, and assemblies without creating a
second source of truth.

## Vertical Slice scope

Must prove named people, work plans, production/consumption, seasonal
bottlenecks, fatigue, trade, delivery time, contracts, political
demands, absence protection, and chronicle.

Out of scope: direct PvP, raids/combat, transport professions,
escorts/ships, detailed religion, marriage politics, complex
inheritance/full succession, large map, multiple major regions,
rankings/chat/clans, resource/profession sprawl, detailed
spoilage/combat/weather.
