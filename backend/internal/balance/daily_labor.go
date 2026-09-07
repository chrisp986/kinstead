package balance

import (
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/simulation"
)

// DailyLaborV1 is intentionally separate from V03. Values are complete
// nine-hour workday quantities, not legacy tick quantities.
func DailyLaborV1() simulation.DailyLaborConfig {
	return simulation.DailyLaborConfig{
		WorkdayHours: 9,
		ProductionPerWorkday: map[simulation.Season]map[workdomain.Activity]int64{
			simulation.Spring: {workdomain.Agriculture: 12000, workdomain.Fishing: 10000, workdomain.Woodcutting: 8000},
			simulation.Summer: {workdomain.Agriculture: 16000, workdomain.Fishing: 13000, workdomain.Woodcutting: 8500},
			simulation.Autumn: {workdomain.Agriculture: 18000, workdomain.Fishing: 10500, workdomain.Woodcutting: 9000},
			simulation.Winter: {workdomain.Agriculture: 4000, workdomain.Fishing: 6500, workdomain.Woodcutting: 10000},
		},
		ConsumptionRules:      simulation.ConsumptionConfig{PerAdultPerDayMilli: 600, PerChildPerDayMilli: 350, ChildAgeYears: 14},
		WoodUpkeepPerDayMilli: 5000,
		WorkFatigueRules:      simulation.FatigueConfig{WorkDeltaPerHour: 3, ServiceDeltaPerHour: 3, Max: 100},
		RecoveryRules:         simulation.RecoveryConfig{RestDeltaPerHour: 2, Min: 0},
		ReservePolicy:         simulation.ReservePolicyConfig{FoodStopDays: 30, FoodResumeDays: 12, WoodStopDays: 18, WoodResumeDays: 7},
		SkillModifierPermille: 1150,
		FarmModifiers: map[workdomain.Activity]map[workdomain.Activity]int64{
			workdomain.Agriculture: {workdomain.Agriculture: 1150, workdomain.Fishing: 900},
			workdomain.Fishing:     {workdomain.Fishing: 1150, workdomain.Agriculture: 900},
			workdomain.Woodcutting: {workdomain.Woodcutting: 1200, workdomain.Agriculture: 900},
		},
	}
}
