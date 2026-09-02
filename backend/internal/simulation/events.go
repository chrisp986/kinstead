package simulation

func contextForTick(tick int64) TickContext {
	ctx := TickContext{
		Season:                      SeasonForTick(tick),
		AgricultureModifierPermille: 1000,
		FishingModifierPermille:     1000,
	}
	// Deterministic v0.3 event calendar.
	if tick >= 7 && tick <= 9 {
		ctx.FishingModifierPermille = 1100
	}
	if tick >= 31 && tick <= 34 {
		ctx.AgricultureModifierPermille = 850
	}
	return ctx
}
