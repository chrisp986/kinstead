package simulation

import (
	"fmt"

	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
)

type TickResult struct {
	State                   HouseholdState
	ProducedProvisionsMilli int64
	ProducedWoodMilli       int64
	FoodShortageMilli       int64
	Daily                   *DailyLaborResult
}

type HourInterval struct {
	Start calendar.Moment
	End   calendar.Moment
}

// WorkContext is calculated once from the authoritative world interval and is
// shared by execution, previews, and work resolution. Simulation code never
// consults the host clock or invents a daylight rule.
type WorkContext struct {
	Season  calendar.ProductionSeason
	Workday calendar.Workday
}

type DomainFact struct {
	Type string
	Data map[string]any
}

type HourResult struct {
	State                          DailyLaborState
	ProducedProvisionsMilli        int64
	ProducedWoodMilli              int64
	FoodShortageMilli              int64
	Settled                        bool
	SettledProvisionsMilli         int64
	SettledWoodMilli               int64
	ConsumedProvisionsMilli        int64
	ConsumedWoodMilli              int64
	SettledConsumedProvisionsMilli int64
	SettledConsumedWoodMilli       int64
	Facts                          []DomainFact
}

type DailyLaborResult struct {
	State DailyLaborState
	Hour  HourResult
}

func ProcessTick(state HouseholdState, tick int64, assignments []Assignment, ctx TickContext, cfg BalanceConfig) (TickResult, error) {
	if tick != state.Tick+1 {
		return TickResult{}, fmt.Errorf("non-sequential tick: got %d, expected %d", tick, state.Tick+1)
	}
	if ctx.Season == "" || ctx.AgricultureModifierPermille <= 0 || ctx.FishingModifierPermille <= 0 {
		return TickResult{}, fmt.Errorf("invalid tick context")
	}
	assigned := make(map[string]Assignment, len(assignments))
	for _, a := range assignments {
		if _, exists := assigned[a.CharacterID]; exists {
			return TickResult{}, fmt.Errorf("character ID %q assigned twice", a.CharacterID)
		}
		if _, err := state.CharacterIndexByID(a.CharacterID); err != nil {
			return TickResult{}, err
		}
		assigned[a.CharacterID] = a
	}

	var food, wood int64
	for i := range state.Characters {
		c := &state.Characters[i]
		a, ok := assigned[c.ID]
		if !ok {
			a = Assignment{CharacterID: c.ID, Activity: Rest, Intensity: Normal}
		}
		produced := EstimateProduction(*c, a, state.FarmSpecialization, ctx, cfg)
		switch a.Activity {
		case Agriculture, Fishing:
			food += produced
		case Woodcutting:
			wood += produced
		case Building:
			// Building progress is handled by the strategy runner because a build target is strategic state.
		}
		applyFatigue(c, a.Activity, a.Intensity, cfg)
	}

	state.ProvisionsMilli += food
	state.WoodMilli += wood
	state.ProvisionsMilli -= cfg.ConsumptionPerTickMilli
	var shortage int64
	if state.ProvisionsMilli < 0 {
		shortage = -state.ProvisionsMilli
		state.ProvisionsMilli = 0
	}
	state.WoodMilli -= cfg.DailyWoodUpkeepMilli
	if state.WoodMilli < 0 {
		state.WoodMilli = 0
	}

	if state.SupplyBelowGameDays(cfg, cfg.CriticalSupplyDays, ctx.GameDaysPerTickNum, ctx.GameDaysPerTickDen) {
		state.CriticalDays++
	}
	if state.SupplyAtMostGameDays(cfg, cfg.StrainedSupplyDays, ctx.GameDaysPerTickNum, ctx.GameDaysPerTickDen) {
		state.StrainedDays++
	}
	state.Tick = tick
	return TickResult{State: state, ProducedProvisionsMilli: food, ProducedWoodMilli: wood, FoodShortageMilli: shortage}, nil
}

// ProcessHour advances one [start,end) interval for a daily-labor household.
// It has no dependency on persistence and is therefore also used by bounded
// forecasts and deterministic balance scenarios.
// ProcessHour retains the daily_labor_v1 fixed-workday compatibility API.
func ProcessHour(state DailyLaborState, interval HourInterval, cfg DailyLaborConfig) (HourResult, error) {
	return ProcessHourWithContext(state, interval, DailyLaborWorkContext(interval.Start.Day), cfg)
}

func ProcessHourWithContext(state DailyLaborState, interval HourInterval, workContext WorkContext, cfg DailyLaborConfig) (HourResult, error) {
	if interval.End != calendar.AdvanceMoment(interval.Start, 1) {
		return HourResult{}, fmt.Errorf("hour interval must be one hour and end-exclusive")
	}
	if state.Tick < 0 || state.ProvisionsMilli < 0 || state.WoodMilli < 0 {
		return HourResult{}, fmt.Errorf("invalid daily labor state")
	}
	if cfg.MaximumNormalWorkHours <= 0 || cfg.ConsumptionRules.PerAdultPerDayMilli <= 0 || cfg.RecoveryRules.Min < 0 || workContext.Season == "" || workContext.Workday.EndHour < workContext.Workday.StartHour {
		return HourResult{}, fmt.Errorf("invalid daily labor configuration")
	}
	if state.ProductionRemainders == nil {
		state.ProductionRemainders = make(map[string]int64)
	}
	if state.FatigueRemainders == nil {
		state.FatigueRemainders = make(map[string]int64)
	}
	applyOccupationChanges(&state, interval.Start)
	previousPolicyReason := state.PolicyReason
	policy := WorkPolicyDecision{}
	if cfg.AllowReserveRedirects {
		policy = ResolveReservePolicy(state, cfg)
	}
	state.PolicyReason = policy.Reason
	facts := make([]DomainFact, 0)
	if interval.Start.Hour == workContext.Workday.StartHour && policy.Reason != "" && policy.Reason != previousPolicyReason {
		facts = append(facts, DomainFact{Type: "household_protection_changed", Data: map[string]any{"reason": policy.Reason}})
	}
	var producedFood, producedWood int64
	for i := range state.Characters {
		c := &state.Characters[i]
		for _, duty := range c.TemporaryDuties {
			if duty.Starts == interval.Start {
				facts = append(facts, DomainFact{Type: "temporary_duty_started", Data: map[string]any{"character_id": c.ID, "duty_id": duty.ID, "activity": duty.Activity}})
			}
			if duty.Ends == interval.Start {
				facts = append(facts, DomainFact{Type: "temporary_duty_ended", Data: map[string]any{"character_id": c.ID, "duty_id": duty.ID, "activity": duty.Activity}})
			}
		}
		policyActivity, policyReason := policy.ActivityFor(c)
		resolved, err := workdomain.ResolveEffectiveWork(workdomain.WorkResolutionInput{
			Moment: interval.Start, Workday: workContext.Workday, Status: c.Status, LaborPermille: c.LaborPermille,
			Occupation: c.Occupation, TemporaryDuties: c.TemporaryDuties,
			PolicyActivity: policyActivity, PolicyReason: policyReason,
		})
		if err != nil {
			return HourResult{}, fmt.Errorf("resolve work for %s: %w", c.ID, err)
		}
		if resolved.BlockedByDuty {
			if resolved.Activity == workdomain.RulerService {
				c.Fatigue = clampFatigue(c.Fatigue + cfg.WorkFatigueRules.ServiceDeltaPerHour)
			} else {
				c.Fatigue = recoverFatigue(c.Fatigue, cfg.RecoveryRules)
			}
			continue
		}
		if resolved.Working {
			amount := EstimateHourlyProduction(*c, resolved.Activity, Season(workContext.Season), state.FarmSpecialization, cfg)
			key := c.ID + ":" + string(resolved.Activity)
			amount, state.ProductionRemainders[key] = splitRate(amount, state.ProductionRemainders[key], int64(cfg.MaximumNormalWorkHours))
			switch resolved.Activity {
			case workdomain.Agriculture, workdomain.Fishing:
				state.PendingProvisionsMilli += amount
				producedFood += amount
			case workdomain.Woodcutting:
				state.PendingWoodMilli += amount
				producedWood += amount
			}
			c.Fatigue = clampFatigue(c.Fatigue + cfg.WorkFatigueRules.WorkDeltaPerHour)
		} else {
			c.Fatigue = recoverFatigue(c.Fatigue, cfg.RecoveryRules)
		}
	}

	settlesToday := interval.Start.Day == interval.End.Day && interval.End.Hour == workContext.Workday.EndHour
	var settledFood, settledWood int64
	settle := func() {
		if !settlesToday || (state.LastSettlementDay != nil && *state.LastSettlementDay == interval.Start.Day) {
			return
		}
		settledFood, settledWood = state.PendingProvisionsMilli, state.PendingWoodMilli
		state.ProvisionsMilli += settledFood
		state.WoodMilli += settledWood
		state.PendingProvisionsMilli = 0
		state.PendingWoodMilli = 0
		day := interval.Start.Day
		state.LastSettlementDay = &day
	}
	if cfg.SettleBeforeConsumption {
		// The monthly model deposits due output before consumption at this
		// boundary; daily_labor_v1 retains its original compatibility ordering.
		settle()
	}

	consumption := dailyConsumption(state, cfg)
	consumed, nextConsumptionRemainder := splitRate(consumption, state.ConsumptionRemainder, 24)
	state.ConsumptionRemainder = nextConsumptionRemainder
	shortage := int64(0)
	if state.ProvisionsMilli < consumed {
		shortage = consumed - state.ProvisionsMilli
		state.ProvisionsMilli = 0
	} else {
		state.ProvisionsMilli -= consumed
	}
	upkeep, nextWoodRemainder := splitRate(cfg.WoodUpkeepPerDayMilli, state.WoodUpkeepRemainder, 24)
	state.WoodUpkeepRemainder = nextWoodRemainder
	if state.WoodMilli < upkeep {
		state.WoodMilli = 0
	} else {
		state.WoodMilli -= upkeep
	}
	if !cfg.SettleBeforeConsumption {
		settle()
	}

	state.CurrentGameDay = interval.End.Day
	state.Tick++
	var settledConsumedFood, settledConsumedWood int64
	if settlesToday {
		settledConsumedFood, settledConsumedWood = consumption, cfg.WoodUpkeepPerDayMilli
	}
	return HourResult{State: state, ProducedProvisionsMilli: producedFood, ProducedWoodMilli: producedWood, FoodShortageMilli: shortage,
		Settled: settlesToday, SettledProvisionsMilli: settledFood, SettledWoodMilli: settledWood,
		ConsumedProvisionsMilli: consumed, ConsumedWoodMilli: upkeep,
		SettledConsumedProvisionsMilli: settledConsumedFood, SettledConsumedWoodMilli: settledConsumedWood, Facts: facts}, nil
}

func applyOccupationChanges(state *DailyLaborState, start calendar.Moment) {
	for i := range state.Characters {
		o := &state.Characters[i].Occupation
		effectiveHour := 8 // compatibility for pre-migration daily_labor_v1 rows
		if o.EffectiveHour != nil {
			effectiveHour = *o.EffectiveHour
		}
		if o.PendingActivity != nil && o.EffectiveDay != nil && (start.Day > *o.EffectiveDay || (start.Day == *o.EffectiveDay && start.Hour >= effectiveHour)) {
			o.Activity = *o.PendingActivity
			o.PendingActivity = nil
			o.EffectiveDay = nil
			o.EffectiveHour = nil
		}
	}
}

func splitRate(rate, remainder, divisor int64) (int64, int64) {
	if rate <= 0 || divisor <= 0 {
		return 0, remainder
	}
	total := rate + remainder
	return total / divisor, total % divisor
}

func clampFatigue(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func recoverFatigue(value int, cfg RecoveryConfig) int {
	value -= cfg.RestDeltaPerHour
	if value < cfg.Min {
		value = cfg.Min
	}
	return clampFatigue(value)
}

func DailyLaborWorkContext(day calendar.GameDay) WorkContext {
	return WorkContext{Season: calendar.DailyLaborDefinition.ProductionSeasonAt(day), Workday: calendar.Workday{SunriseHour: 8, SunsetHour: 17, StartHour: 8, EndHour: 17}}
}
