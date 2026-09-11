# Monthly-season labor balance

`monthly_seasons_v1` uses fixed-point milli-units (`1000 = 1 unit`). Rates are
defined for a complete eight-hour adult workday before capacity, fatigue,
skill, and farm-specialization modifiers. A short daylight window earns only
the hourly shares that fit; integer remainders remain pending until settlement.

| Season | Agriculture | Fishing | Woodcutting |
| --- | ---: | ---: | ---: |
| Spring | 1,000 | 850 | 5,000 |
| Summer | 1,200 | 1,050 | 5,200 |
| Autumn | 1,300 | 900 | 5,800 |
| Winter | 850 | 1,100 | 7,000 |

Daily requirements are 600 provisions per supported adult, 350 per child
(under age 14), and 5,000 wood for household upkeep. Work fatigue is 3 points
per hour, recovery is 2 points per non-working hour, and fatigue is bounded to
0--100. Skill contributes a 1.15 multiplier in the initial seed.

The new model does not redirect an ordinary occupation in response to reserve
levels. A farmer remains a farmer, a fisher remains a fisher, and a woodcutter
remains a woodcutter. Pending output is not spendable until that date's
calculated work-end settlement. Temporary commitments can suspend home output
without deleting the occupation.

The deterministic starting-household scenario begins at world midnight on
January 1, 2028 and runs through three complete real-calendar years (1,096
days, including leap-year February). It uses 100,000 provisions, 60,000 wood,
two adult food workers, one adult woodworker, one half-capacity adolescent, and
one non-working child. The checked-in test requires no food-shortage hours,
positive wood throughout, and stable fatigue. Exact ending-stock observations
are recorded by the test log; the current result ends at 316,675 provisions
and 431,900 wood, with maximum fatigue 24 (all resource values are milli-units).
Upper bounds also guard against restoring the earlier runaway reserve growth.

Forecasts assume the persisted occupations and supported temporary
commitments continue through the bounded horizon. They include confirmed
incoming shipments at their execution boundary, recalculate season and
daylight every hour, and report snapshot identity, assumptions, and warnings.
They cannot predict future player commands or unconfirmed trade.

These tests establish deterministic viability, not engagement or final
economy balance. Playtesting must still determine whether occupation choices
are clear and meaningful, whether the Work Plan explains interruptions, and
whether reports create a satisfying reason to return.

Crop mechanics are intentionally absent. Fields, seeds, sowing, crop growth,
tending, harvesting, and seasonal food-gap planning belong to the deferred
seasonal-agriculture milestone.
