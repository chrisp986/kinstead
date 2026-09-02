package simulation

import "fmt"

type ScenarioSummary struct {
	Strategy             StrategyName
	SupplyDays           float64
	Silver               float64
	Wood                 float64
	StrainedDays         int
	CriticalDays         int
	TradeVolume          float64
	JarlStanding         int
	PoliticalServiceDays int
	PoliticalWoodPaid    float64
	PoliticalSilverPaid  float64
	Buildings            []string
}

func RunV03Scenario(strategy StrategyName) (ScenarioSummary, error) {
	cfg := DefaultBalanceConfig()
	state := NewBjornvikState()
	ss := StrategyState{Name: strategy, ServiceCharacter: "Einar"}

	for tick := int64(1); tick <= 48; tick++ {
		// Decisions happen before work for this technical scenario runner.
		applyPoliticalEvent(&state, &ss, tick, cfg)
		applyTradeRules(&state, strategy, tick, cfg)
		assignments := chooseAssignments(state, ss, cfg)
		startAndProgressBuilding(&state, strategy, assignments)
		result, err := ProcessTick(state, tick, assignments, cfg)
		if err != nil {
			return ScenarioSummary{}, err
		}
		state = result.State
		if ss.ServiceRemaining > 0 {
			ss.ServiceRemaining--
			state.PoliticalServiceDays++
		}
	}

	buildings := []string{}
	for _, b := range state.Buildings {
		if b.Completed {
			buildings = append(buildings, b.Name)
		}
	}
	return ScenarioSummary{
		Strategy:             strategy,
		SupplyDays:           state.SupplyDays(cfg),
		Silver:               float64(state.SilverMilli) / 1000,
		Wood:                 float64(state.WoodMilli) / 1000,
		StrainedDays:         state.StrainedDays,
		CriticalDays:         state.CriticalDays,
		TradeVolume:          float64(state.TradeVolumeMilli) / 1000,
		JarlStanding:         state.JarlStanding,
		PoliticalServiceDays: state.PoliticalServiceDays,
		PoliticalWoodPaid:    float64(state.PoliticalWoodPaidMilli) / 1000,
		PoliticalSilverPaid:  float64(state.PoliticalSilverPaidMilli) / 1000,
		Buildings:            buildings,
	}, nil
}

func RunAllV03Scenarios() ([]ScenarioSummary, error) {
	out := make([]ScenarioSummary, 0, 5)
	for _, strategy := range allStrategies() {
		s, err := RunV03Scenario(strategy)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", strategy, err)
		}
		out = append(out, s)
	}
	return out, nil
}

func applyPoliticalEvent(state *HouseholdState, ss *StrategyState, tick int64, cfg BalanceConfig) {
	supply := state.SupplyDays(cfg)
	switch tick {
	case 15:
		serve := (ss.Name == StrategyAutark || ss.Name == StrategySupplySafe || ss.Name == StrategyLoyal) && supply >= 20
		if serve {
			ss.ServiceRemaining = 4
			state.JarlStanding += 10
		} else {
			state.JarlStanding -= 5
		}
	case 28:
		switch ss.Name {
		case StrategyLoyal:
			if state.WoodMilli >= 18_000 {
				state.WoodMilli -= 18_000
				state.PoliticalWoodPaidMilli += 18_000
				state.JarlStanding += 10
			} else if state.SilverMilli >= 6_000 {
				state.SilverMilli -= 6_000
				state.PoliticalSilverPaidMilli += 6_000
				state.JarlStanding += 10
			} else {
				state.JarlStanding -= 5
			}
		case StrategyTrader:
			if state.SilverMilli >= 30_000 {
				state.SilverMilli -= 6_000
				state.PoliticalSilverPaidMilli += 6_000
				state.JarlStanding += 10
			} else {
				state.JarlStanding -= 5
			}
		default:
			state.JarlStanding -= 5
		}
	case 42:
		if ss.Name == StrategyLoyal && supply >= 15 {
			ss.ServiceRemaining = 4
			state.JarlStanding += 10
		} else {
			state.JarlStanding -= 5
		}
	}
}

func applyTradeRules(state *HouseholdState, strategy StrategyName, tick int64, cfg BalanceConfig) {
	buyFood := func(units int64, transportSilver int64) bool {
		cost := units*500 + transportSilver*1000
		if state.SilverMilli < cost {
			return false
		}
		state.SilverMilli -= cost
		state.ProvisionsMilli += units * 1000
		state.TradeVolumeMilli += units * 1000
		return true
	}
	sellFood := func(units int64) bool {
		if state.ProvisionsMilli < units*1000 {
			return false
		}
		state.ProvisionsMilli -= units * 1000
		state.SilverMilli += units * 500
		state.TradeVolumeMilli += units * 1000
		return true
	}
	sellGoods := func(units int64, premium bool) bool {
		if state.TradeGoodsMilli < units*1000 {
			return false
		}
		price := int64(5000)
		if premium {
			price = 5750
		}
		state.TradeGoodsMilli -= units * 1000
		state.SilverMilli += units * price
		state.TradeVolumeMilli += units * 1000
		return true
	}
	buyWood := func(units int64) bool {
		cost := units * 600 // 10 wood = 6 silver
		if state.SilverMilli < cost {
			return false
		}
		state.SilverMilli -= cost
		state.WoodMilli += units * 1000
		state.TradeVolumeMilli += units * 1000
		return true
	}

	supply := state.SupplyDays(cfg)
	switch strategy {
	case StrategySupplySafe:
		if supply < 16 {
			buyFood(10, 0)
		}
	case StrategyTrader:
		if supply < 16 {
			buyFood(30, 2)
		}
		if tick == 21 {
			sellGoods(4, true)
		}
		if state.SupplyDays(cfg) > 29 && state.ProvisionsMilli >= 20_000 {
			sellFood(10)
		}
		if state.WoodMilli < 30_000 && state.SilverMilli >= 35_000 {
			buyWood(20)
		}
	case StrategyBuilder:
		if supply < 18 {
			buyFood(35, 1)
		}
		if tick == 1 && state.SilverMilli < 35_000 {
			sellGoods(2, false)
		}
		if nextIncompleteBuilding(state) != nil && state.WoodMilli < nextIncompleteBuilding(state).WoodCostMilli && state.SilverMilli >= 25_000 {
			buyWood(20)
		}
	}
}

func nextIncompleteBuilding(state *HouseholdState) *BuildingState {
	for i := range state.Buildings {
		if !state.Buildings[i].Completed {
			return &state.Buildings[i]
		}
	}
	return nil
}

func maxBuildings(strategy StrategyName) int {
	if strategy == StrategyBuilder {
		return 3
	}
	if strategy == StrategyTrader {
		return 2
	}
	return 2
}

func startAndProgressBuilding(state *HouseholdState, strategy StrategyName, assignments []Assignment) {
	completed := 0
	for _, b := range state.Buildings {
		if b.Completed {
			completed++
		}
	}
	if completed >= maxBuildings(strategy) {
		return
	}
	b := nextIncompleteBuilding(state)
	if b == nil {
		return
	}
	if !b.Started {
		if state.WoodMilli < b.WoodCostMilli {
			return
		}
		state.WoodMilli -= b.WoodCostMilli
		b.Started = true
	}
	// Construction competes with productive work: only an explicit building assignment progresses the project.
	for _, a := range assignments {
		if a.Activity == Building {
			idx, _ := state.CharacterIndex(a.Character)
			b.ProgressPermille += state.Characters[idx].LaborPermille
			break
		}
	}
	if b.ProgressPermille >= b.WorkerDaysPermille {
		b.Completed = true
	}
}
