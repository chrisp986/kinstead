package simulation

import "fmt"

type TickResult struct {
	State                   HouseholdState
	ProducedProvisionsMilli int64
	ProducedWoodMilli       int64
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
		if _, exists := assigned[a.Character]; exists {
			return TickResult{}, fmt.Errorf("character %q assigned twice", a.Character)
		}
		if _, err := state.CharacterIndex(a.Character); err != nil {
			return TickResult{}, err
		}
		assigned[a.Character] = a
	}

	var food, wood int64
	for i := range state.Characters {
		c := &state.Characters[i]
		a, ok := assigned[c.Name]
		if !ok {
			a = Assignment{Character: c.Name, Activity: Rest, Intensity: Normal}
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
	state.ProvisionsMilli -= cfg.DailyConsumptionMilli
	if state.ProvisionsMilli < 0 {
		state.ProvisionsMilli = 0
	}
	state.WoodMilli -= cfg.DailyWoodUpkeepMilli
	if state.WoodMilli < 0 {
		state.WoodMilli = 0
	}

	if state.SupplyDays(cfg) < float64(cfg.CriticalSupplyDays) {
		state.CriticalDays++
	}
	if state.SupplyDays(cfg) <= float64(cfg.StrainedSupplyDays) {
		state.StrainedDays++
	}
	state.Tick = tick
	return TickResult{State: state, ProducedProvisionsMilli: food, ProducedWoodMilli: wood}, nil
}
