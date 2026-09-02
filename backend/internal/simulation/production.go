package simulation

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

func productionFor(c Character, a Assignment, farmSpecialization Activity, ctx TickContext, cfg BalanceConfig) int64 {
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
