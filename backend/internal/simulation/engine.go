package simulation

import "fmt"

type TickResult struct {
	State                   HouseholdState
	ProducedProvisionsMilli int64
	ProducedWoodMilli       int64
	FoodShortageMilli       int64
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
