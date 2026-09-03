package simulation

type IntensityRule struct {
	ProductionPermille int64
	FatigueDelta       int
}

// BalanceConfig contains gameplay mechanics consumed by the generic engine.
// Concrete balance versions live outside this package.
type BalanceConfig struct {
	DailyConsumptionMilli int64
	DailyWoodUpkeepMilli  int64
	CriticalSupplyDays    int64
	EmergencySupplyDays   int64
	StrainedSupplyDays    int64
	Intensity             map[Intensity]IntensityRule
	Production            map[Season]map[Activity]int64 // milli-units per full worker per simulation tick
	FarmModifiers         map[Activity]map[Activity]int64
	SkillModifierPermille int64
}
