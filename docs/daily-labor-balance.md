# Daily-labor balance

`daily_labor_v1` uses fixed-point milli-units (`1000 = 1 unit`) and a nine-hour
workday. These are the initial coefficients per complete adult workday before
capacity, fatigue, skill, and household policy modifiers:

| Season | Agriculture | Fishing | Woodcutting |
| --- | ---: | ---: | ---: |
| Spring | 12,000 | 10,000 | 8,000 |
| Summer | 16,000 | 13,000 | 8,500 |
| Autumn | 18,000 | 10,500 | 9,000 |
| Winter | 4,000 | 6,500 | 10,000 |

Daily requirements are 600 provisions per supported adult, 350 per child
(under age 14), and 5,000 wood for household upkeep. Work fatigue is 3 points
per hour, recovery is 2 points per non-working hour, and fatigue is bounded to
0--100. Skill contributes a 1.15 multiplier in the initial seed.

The reserve policy stops ordinary food work at 30 days of provisions and
resumes it below 12 days; wood stops at 18 days of upkeep and resumes below 7
days. Pending earned output is considered by the policy but is not spendable
until the 17:00 settlement. Hysteresis limits oscillation and preserves the
character's occupation.

The deterministic three-year starting-household scenario uses 100,000
provisions, 60,000 wood, two adult food workers, one adult woodworker, one
half-capacity adolescent, and one non-working child. It completes three
365-day years with zero food shortages, covered wood upkeep, maximum fatigue
of 27, a minimum of 72,503 provisions and 56,667 wood, and ending reserves
of 83,366 provisions and 89,942 wood (all values milli-units). This baseline
does not establish final balance or player engagement; seasonal and intraday
shortages remain playtest questions.

Crop mechanics are intentionally absent. Fields, seeds, sowing, crop growth,
tending, harvesting, and seasonal food-gap planning belong to the deferred
seasonal-agriculture milestone.
