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

Target \~5-minute sessions and 1--2 sessions/day. Several days of
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

Production explicitly maps wall time → simulation ticks → historical
date. The v0.3 balancing model's `48 ticks/year, 12/season` is synthetic
and must not define production calendar semantics.

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

Communicate supply as "Provisions last X days." Thresholds: \>30 safe;
15--30 strained; \<15 critical; \<7 emergency.

Emergency AI should pause nonessential construction, stop high-intensity
work, shift labor toward food, sell limited reserves, and buy NPC food
if needed. Preserve \~10 silver where possible. Log all automatic
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

Contract trust consequences:

- fulfilled on time or early: +2
- fulfilled 1 tick late: -1
- fulfilled 2 ticks late: -2
- broken / 3+ ticks late: -8

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

Contract trust consequences are fixed and directional: fulfilled on time or
early is +2, one tick late is -1, two ticks late is -2, and broken/3+ ticks
late is -8. Each obligation produces at most one consequence; trust is clamped
to -100..100 and does not scale by quantity, value, resource type, distance,
or contract duration.

Jarl demands use production event types `political_labor_service` and
`political_levy`. Labor service reserves one full-capacity character for four
ticks. Levy honor costs 18,000 wood or 6,000 silver milli-units. Honoring is
+10 political score; refusal or expiry default is -5. Political standing is
derived from score and is never persisted separately.

Chronicle stores structured facts and renders prose later. Record major
trades, contracts, buildings, political choices, shipment issues, and
emergency actions.

## UI

Primary screens: 1. Farm Report 2. Farm 3. Work Planning 4. Trade 5.
Chronicle

Farm Report answers: What changed? What needs attention? What meaningful
decisions matter now? Prefer 2--3 important actions.

## Vertical Slice scope

Must prove named people, work plans, production/consumption, seasonal
bottlenecks, fatigue, trade, delivery time, contracts, political
demands, absence protection, and chronicle.

Out of scope: direct PvP, raids/combat, transport professions,
escorts/ships, detailed religion, marriage politics, complex
inheritance/full succession, large map, multiple major regions,
rankings/chat/clans, resource/profession sprawl, detailed
spoilage/combat/weather.
