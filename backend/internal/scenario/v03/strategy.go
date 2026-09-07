package v03

import "sort"

type StrategyName string

const (
	StrategyAutark     StrategyName = "autark"
	StrategySupplySafe StrategyName = "supply_safe"
	StrategyTrader     StrategyName = "trader"
	StrategyBuilder    StrategyName = "builder"
	StrategyLoyal      StrategyName = "loyal"
)

type StrategyState struct {
	Name             StrategyName
	ServiceRemaining int
	ServiceCharacter string
}

func allStrategies() []StrategyName {
	return []StrategyName{StrategyAutark, StrategySupplySafe, StrategyTrader, StrategyBuilder, StrategyLoyal}
}

func bestFoodActivity(c Character, farmSpecialization Activity, season Season, cfg BalanceConfig) Activity {
	ctx := TickContext{Season: season, AgricultureModifierPermille: 1000, FishingModifierPermille: 1000, GameDaysPerTickNum: 1, GameDaysPerTickDen: 1}
	fa := EstimateProduction(c, Assignment{CharacterID: c.ID, Activity: Agriculture, Intensity: Normal}, farmSpecialization, ctx, cfg)
	ff := EstimateProduction(c, Assignment{CharacterID: c.ID, Activity: Fishing, Intensity: Normal}, farmSpecialization, ctx, cfg)
	if ff >= fa {
		return Fishing
	}
	return Agriculture
}

func assignFood(assignments *[]Assignment, c Character, farmSpecialization Activity, season Season, intensity Intensity, cfg BalanceConfig) {
	*assignments = append(*assignments, Assignment{CharacterID: c.ID, Activity: bestFoodActivity(c, farmSpecialization, season, cfg), Intensity: intensity})
}

func chooseAssignments(state HouseholdState, ss StrategyState, cfg BalanceConfig) []Assignment {
	season := SeasonForTick(state.Tick + 1)
	chars := append([]Character(nil), state.Characters...)
	// Full workers first; half-capacity apprentice last.
	sort.SliceStable(chars, func(i, j int) bool { return chars[i].LaborPermille > chars[j].LaborPermille })
	used := map[string]bool{}
	out := []Assignment{}

	if ss.ServiceRemaining > 0 {
		id := ss.ServiceCharacter
		if id == "" {
			id = "einar"
		}
		out = append(out, Assignment{CharacterID: id, Activity: RulerService, Intensity: Normal})
		used[id] = true
	}

	supply := syntheticSupplyDays(state, cfg)
	foodWorkers := 1
	switch ss.Name {
	case StrategyAutark:
		if supply < 30 {
			foodWorkers = 2
		}
	case StrategySupplySafe:
		if supply < 40 {
			foodWorkers = 2
		} else {
			foodWorkers = 1
		}
	case StrategyTrader:
		if supply < 20 {
			foodWorkers = 2
		} else {
			foodWorkers = 1
		}
	case StrategyBuilder:
		if supply < 23 {
			foodWorkers = 2
		} else {
			foodWorkers = 1
		}
	case StrategyLoyal:
		if supply < 25 {
			foodWorkers = 2
		} else {
			foodWorkers = 1
		}
	}

	for _, c := range chars {
		if c.LaborPermille == 0 || used[c.ID] || foodWorkers <= 0 {
			continue
		}
		assignFood(&out, c, state.FarmSpecialization, season, Normal, cfg)
		used[c.ID] = true
		foodWorkers--
	}

	// If a building can be worked on, reserve one worker for construction.
	if b := nextIncompleteBuilding(&state); b != nil && maxBuildings(ss.Name) > completedBuildingCount(state) && (b.Started || state.WoodMilli >= b.WoodCostMilli) {
		for _, c := range chars {
			if c.LaborPermille == 0 || used[c.ID] {
				continue
			}
			out = append(out, Assignment{CharacterID: c.ID, Activity: Building, Intensity: Normal})
			used[c.ID] = true
			break
		}
	}

	// Strategic use of remaining labor.
	for _, c := range chars {
		if c.LaborPermille == 0 || used[c.ID] {
			continue
		}
		activity := Woodcutting
		switch ss.Name {
		case StrategyTrader:
			// Trader monetizes the farm's fishing specialization.
			activity = Fishing
		case StrategyBuilder:
			if allBuildingsComplete(state) && state.WoodMilli >= 80_000 {
				activity = bestFoodActivity(c, state.FarmSpecialization, season, cfg)
			}
		case StrategyLoyal:
			if completedBuildingCount(state) >= maxBuildings(ss.Name) && state.WoodMilli >= 60_000 {
				activity = bestFoodActivity(c, state.FarmSpecialization, season, cfg)
			}
		case StrategySupplySafe:
			if completedBuildingCount(state) >= maxBuildings(ss.Name) && state.WoodMilli >= 25_000 {
				activity = bestFoodActivity(c, state.FarmSpecialization, season, cfg)
			}
		case StrategyAutark:
			if completedBuildingCount(state) >= maxBuildings(ss.Name) && state.WoodMilli >= 40_000 {
				activity = bestFoodActivity(c, state.FarmSpecialization, season, cfg)
			}
		}
		out = append(out, Assignment{CharacterID: c.ID, Activity: activity, Intensity: Normal})
		used[c.ID] = true
	}
	return out
}

func allBuildingsComplete(state HouseholdState) bool {
	for _, b := range state.Buildings {
		if !b.Completed {
			return false
		}
	}
	return true
}

func completedBuildingCount(state HouseholdState) int {
	n := 0
	for _, b := range state.Buildings {
		if b.Completed {
			n++
		}
	}
	return n
}
