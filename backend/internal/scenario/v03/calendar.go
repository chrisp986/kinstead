package v03

// SeasonForTick is deliberately local to the frozen v0.3 balancing model.
// Its 48-tick year is not a production historical calendar.
func SeasonForTick(tick int64) Season {
	if tick <= 0 {
		return Spring
	}
	period := ((tick - 1) % 48) + 1
	switch {
	case period <= 12:
		return Spring
	case period <= 24:
		return Summer
	case period <= 36:
		return Autumn
	default:
		return Winter
	}
}

func contextForTick(tick int64) TickContext {
	ctx := TickContext{Season: SeasonForTick(tick), AgricultureModifierPermille: 1000, FishingModifierPermille: 1000}
	// Frozen deterministic v0.3 event calendar.
	if tick >= 7 && tick <= 9 {
		ctx.FishingModifierPermille = 1100
	}
	if tick >= 31 && tick <= 34 {
		ctx.AgricultureModifierPermille = 850
	}
	return ctx
}
