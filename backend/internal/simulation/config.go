package simulation

import workdomain "game/backend/internal/domain/work"

type IntensityRule struct {
	ProductionPermille int64
	FatigueDelta       int
}

// BalanceConfig contains gameplay mechanics consumed by the generic engine.
// Concrete balance versions live outside this package.
type BalanceConfig struct {
	ConsumptionPerTickMilli int64
	DailyWoodUpkeepMilli    int64
	CriticalSupplyDays      int64
	EmergencySupplyDays     int64
	StrainedSupplyDays      int64
	Intensity               map[Intensity]IntensityRule
	Production              map[Season]map[Activity]int64 // milli-units per full worker per simulation tick
	FarmModifiers           map[Activity]map[Activity]int64
	SkillModifierPermille   int64
}

type ConsumptionConfig struct {
	PerAdultPerDayMilli int64
	PerChildPerDayMilli int64
	ChildAgeYears       int
}

type FatigueConfig struct {
	WorkDeltaPerHour    int
	ServiceDeltaPerHour int
	Max                 int
}

type RecoveryConfig struct {
	RestDeltaPerHour int
	Min              int
}

type ReservePolicyConfig struct {
	FoodStopDays   int64
	FoodResumeDays int64
	WoodStopDays   int64
	WoodResumeDays int64
}

type DailyLaborConfig struct {
	WorkdayHours          int
	ProductionPerWorkday  map[Season]map[workdomain.Activity]int64
	ConsumptionRules      ConsumptionConfig
	WoodUpkeepPerDayMilli int64
	WorkFatigueRules      FatigueConfig
	RecoveryRules         RecoveryConfig
	ReservePolicy         ReservePolicyConfig
	SkillModifierPermille int64
	FarmModifiers         map[workdomain.Activity]map[workdomain.Activity]int64
}
