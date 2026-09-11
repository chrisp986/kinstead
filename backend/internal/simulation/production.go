package simulation

import (
	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
)

func fatigueProductionPermille(fatigue int) int64 {
	switch {
	case fatigue >= 85:
		return 750
	case fatigue >= 70:
		return 900
	default:
		return 1000
	}
}

func FatigueProductionPermille(fatigue int) int64 { return fatigueProductionPermille(fatigue) }

// EstimateProduction exposes the shared deterministic production mechanic to
// production orchestration and balancing scenarios. Calendar/event selection
// remains outside the generic engine.
func EstimateProduction(c Character, a Assignment, farmSpecialization Activity, ctx TickContext, cfg BalanceConfig) int64 {
	if c.LaborPermille <= 0 {
		return 0
	}
	base := cfg.Production[ctx.Season][a.Activity]
	if base == 0 {
		return 0
	}

	result := base
	result = result * c.LaborPermille / 1000
	result = result * cfg.Intensity[a.Intensity].ProductionPermille / 1000
	result = result * fatigueProductionPermille(c.Fatigue) / 1000

	if modifiers, ok := cfg.FarmModifiers[farmSpecialization]; ok {
		if modifier, ok := modifiers[a.Activity]; ok {
			result = result * modifier / 1000
		}
	}
	if a.Activity == Fishing {
		result = result * ctx.FishingModifierPermille / 1000
	}
	if a.Activity == Agriculture {
		result = result * ctx.AgricultureModifierPermille / 1000
	}
	if c.Specialization == a.Activity {
		result = result * cfg.SkillModifierPermille / 1000
	}
	return result
}

// EstimateHourlyProduction returns a complete normal-workday rate before the
// engine divides it into fixed-point hourly shares. Short winter days remain
// partial instead of awarding a full day's output in fewer hours.
func EstimateHourlyProduction(c DailyCharacter, activity workdomain.Activity, season Season, farmSpecialization Activity, cfg DailyLaborConfig) int64 {
	if c.LaborPermille <= 0 || c.Status == "dead" {
		return 0
	}
	base := cfg.ProductionPerWorkday[season][activity]
	if base <= 0 {
		return 0
	}
	result := base * c.LaborPermille / 1000
	result = result * dailyFatigueProductionPermille(c.Fatigue) / 1000
	if cfg.SkillModifierPermille > 0 && string(c.Specialization) == string(activity) {
		result = result * cfg.SkillModifierPermille / 1000
	}
	if modifiers, ok := cfg.FarmModifiers[workdomain.Activity(farmSpecialization)]; ok {
		if modifier, ok := modifiers[activity]; ok {
			result = result * modifier / 1000
		}
	}
	return result
}

func dailyFatigueProductionPermille(fatigue int) int64 {
	switch {
	case fatigue >= 85:
		return 750
	case fatigue >= 70:
		return 900
	default:
		return 1000
	}
}

// HourlyProductionForMoment is a small convenience for previews and tests.
func HourlyProductionForMoment(c DailyCharacter, activity workdomain.Activity, day calendar.GameDay, cfg DailyLaborConfig) int64 {
	return EstimateHourlyProduction(c, activity, Season(calendar.DailyLaborDefinition.ProductionSeasonAt(day)), c.Specialization, cfg)
}
