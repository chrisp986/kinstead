package simulation

func applyFatigue(c *Character, activity Activity, intensity Intensity, cfg BalanceConfig) {
	if activity == Rest {
		c.Fatigue -= 12
	} else if activity == RulerService {
		c.Fatigue += cfg.Intensity[Normal].FatigueDelta
	} else {
		c.Fatigue += cfg.Intensity[intensity].FatigueDelta
	}
	if c.Fatigue < 0 {
		c.Fatigue = 0
	}
	if c.Fatigue > 100 {
		c.Fatigue = 100
	}
}
