package balance

import "game/backend/internal/simulation"

// V03 returns the frozen v0.3 gameplay coefficients. Production and the
// balancing runner share these mechanics, while their calendars remain
// separate.
func V03() simulation.BalanceConfig {
	return simulation.BalanceConfig{
		DailyConsumptionMilli: 4900,
		DailyWoodUpkeepMilli:  1000,
		CriticalSupplyDays:    15,
		EmergencySupplyDays:   7,
		StrainedSupplyDays:    30,
		Intensity: map[simulation.Intensity]simulation.IntensityRule{
			simulation.Light:  {ProductionPermille: 800, FatigueDelta: 2},
			simulation.Normal: {ProductionPermille: 1000, FatigueDelta: 4},
			simulation.High:   {ProductionPermille: 1200, FatigueDelta: 7},
		},
		Production: map[simulation.Season]map[simulation.Activity]int64{
			simulation.Spring: {simulation.Agriculture: 2500, simulation.Fishing: 3000, simulation.Woodcutting: 3000},
			simulation.Summer: {simulation.Agriculture: 3500, simulation.Fishing: 4000, simulation.Woodcutting: 3000},
			simulation.Autumn: {simulation.Agriculture: 4000, simulation.Fishing: 3000, simulation.Woodcutting: 3000},
			simulation.Winter: {simulation.Agriculture: 1000, simulation.Fishing: 1500, simulation.Woodcutting: 3000},
		},
		FarmModifiers: map[simulation.Activity]map[simulation.Activity]int64{
			simulation.Fishing:     {simulation.Fishing: 1150, simulation.Agriculture: 900},
			simulation.Agriculture: {simulation.Agriculture: 1150, simulation.Fishing: 900},
			simulation.Woodcutting: {simulation.Woodcutting: 1200, simulation.Agriculture: 900},
		},
		SkillModifierPermille: 1150,
	}
}
