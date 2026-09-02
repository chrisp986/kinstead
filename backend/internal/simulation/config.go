package simulation

type IntensityRule struct {
	ProductionPermille int64
	FatigueDelta       int
}

type BalanceConfig struct {
	DailyConsumptionMilli int64
	DailyWoodUpkeepMilli  int64
	CriticalSupplyDays    int64
	EmergencySupplyDays   int64
	StrainedSupplyDays    int64
	Intensity             map[Intensity]IntensityRule
	Production            map[Season]map[Activity]int64 // milli units per full worker-day
	FarmModifiers         map[Activity]map[Activity]int64
	SkillModifierPermille int64
}

func DefaultBalanceConfig() BalanceConfig {
	return BalanceConfig{
		DailyConsumptionMilli: 4900,
		DailyWoodUpkeepMilli:  1000,
		CriticalSupplyDays:    15,
		EmergencySupplyDays:   7,
		StrainedSupplyDays:    30,
		Intensity: map[Intensity]IntensityRule{
			Light:  {ProductionPermille: 800, FatigueDelta: 2},
			Normal: {ProductionPermille: 1000, FatigueDelta: 4},
			High:   {ProductionPermille: 1200, FatigueDelta: 7},
		},
		Production: map[Season]map[Activity]int64{
			Spring: {Agriculture: 2500, Fishing: 3000, Woodcutting: 3000},
			Summer: {Agriculture: 3500, Fishing: 4000, Woodcutting: 3000},
			Autumn: {Agriculture: 4000, Fishing: 3000, Woodcutting: 3000},
			Winter: {Agriculture: 1000, Fishing: 1500, Woodcutting: 3000},
		},
		FarmModifiers: map[Activity]map[Activity]int64{
			Fishing:     {Fishing: 1150, Agriculture: 900},
			Agriculture: {Agriculture: 1150, Fishing: 900},
			Woodcutting: {Woodcutting: 1200, Agriculture: 900},
		},
		SkillModifierPermille: 1150,
	}
}
