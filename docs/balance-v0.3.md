# Balance Baseline v0.3

Status: **Frozen deterministic reference**. Change only during explicit
balancing work.

The 48-tick year is a balancing abstraction, not production historical
time.

## Calendar

48 ticks/year; 12/season: - Spring 1--12 - Summer 13--24 - Autumn
25--36 - Winter 37--48

Events: - tick 7: +10% fishing for 3 ticks - 15: Jarl labor service (4
ticks) - 21: trade-good price +15% for 5 ticks - 28: Jarl levy (18 wood
or 6 silver) - 31: agriculture -15% for 4 ticks - 38: storm, +1 shipment
tick - 42: Jarl labor service (4 ticks) - 48: year end

## Starting resources

-   provisions 150
-   wood 20
-   trade goods 4
-   silver 30

Aggregate consumption: 4.9 provisions/tick. Wood upkeep: 1.0/tick.

Note: older character-level consumption totals 4.1/day. Reconcile before
replacing the aggregate baseline.

## Production

Per full worker/tick:

  Activity        Spring   Summer   Autumn   Winter
  ------------- -------- -------- -------- --------
  Agriculture        2.5      3.5      4.0      1.0
  Fishing            3.0      4.0      3.0      1.5

Wood: 3.0/tick.

Matching character specialization: +15%.

Farm specializations: - agriculture: agriculture +15%, fishing -10% -
fishing: fishing +15%, agriculture -10% - forest: wood +20%, agriculture
-10%

Björnvik balancing scenario currently uses fishing specialization.

## Fatigue/intensity

-   light: production 80%, fatigue +2
-   normal: 100%, +4
-   high: 120%, +7
-   rest: -12

Fatigue penalties: - 0--49: none - 50--69: warning - 70--84: -10% -
85--100: -25%

## Supply

-   30 days safe

-   15--30 strained

-   \<15 critical

-   \<7 emergency

## Trade references

NPC reference: - 10 provisions = 5 silver - 10 wood = 6 silver - 1 trade
good = 5 silver

Deterministic baseline: NPC sells +20%; NPC buys -20%.

Distance ticks: neighbor 1, local 2, near regional 3, regional 5, far
regional 8, long-distance 12. Transport cost: neighbor 0, local 1, near
regional 2, regional 3, far regional 5 silver. Capacity: 100 resource
units or 20 trade goods.

## Buildings

-   Storage: 30 wood + 6 worker-ticks
-   Workshop: 40 wood + 8 worker-ticks
-   Housing: 50 wood + 10 worker-ticks

## Political outcomes

Reference deterministic outcomes: - Autark: Jarl 0; 4 service ticks -
Supply-safe: 0; 4 service ticks - Trader: 0; 6 silver - Builder: -15;
refuses - Loyal: +30; 8 service ticks +18 wood

## Reference end states

  --------------------------------------------------------------------------------
  Strategy          Supply     Silver       Wood      Trade       Jarl Buildings
                      days                           volume            
  ------------- ---------- ---------- ---------- ---------- ---------- -----------
  Autark              37.9       30.0       43.0          0          0 storage,
                                                                       workshop

  Supply-safe         44.0       30.0       28.0          0          0 storage,
                                                                       workshop

  Trader              27.4       52.0       62.0        252          0 workshop,
                                                                       storage

  Builder             21.4       24.9       81.5       45.1        -15 storage,
                                                                       workshop,
                                                                       housing

  Loyal               28.0       30.0       64.0          0        +30 storage,
                                                                       workshop
  --------------------------------------------------------------------------------

These agents are balancing probes, not production AI architecture. Monte
Carlo work is deferred.
