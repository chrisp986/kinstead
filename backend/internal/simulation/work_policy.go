package simulation

import (
	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
)

type WorkPolicyDecision struct {
	RestAll      bool
	RedirectFood bool
	RedirectWood bool
	Reason       string
}

func (d WorkPolicyDecision) ActivityFor(c *DailyCharacter) (*workdomain.Activity, string) {
	if c == nil {
		return nil, ""
	}
	if d.RestAll {
		a := workdomain.Rest
		return &a, d.Reason
	}
	if d.RedirectFood && c.Occupation.Activity == workdomain.Woodcutting {
		a := workdomain.Fishing
		return &a, d.Reason
	}
	if d.RedirectWood && c.Occupation.Activity != workdomain.Woodcutting {
		a := workdomain.Woodcutting
		return &a, d.Reason
	}
	return nil, ""
}

// ResolveReservePolicy is deliberately a bounded, hysteresis-based policy.
// It considers spendable and pending resources but never converts a
// character's persistent occupation into an assignment.
func ResolveReservePolicy(state DailyLaborState, cfg DailyLaborConfig) WorkPolicyDecision {
	consumption := dailyConsumption(state, cfg)
	if consumption <= 0 {
		return WorkPolicyDecision{}
	}
	foodDays := (state.ProvisionsMilli + state.PendingProvisionsMilli) / consumption
	woodDays := int64(0)
	if cfg.WoodUpkeepPerDayMilli > 0 {
		woodDays = (state.WoodMilli + state.PendingWoodMilli) / cfg.WoodUpkeepPerDayMilli
	}
	if foodDays <= cfg.ReservePolicy.FoodResumeDays {
		return WorkPolicyDecision{RedirectFood: true, Reason: "food reserves need replenishing"}
	}
	if woodDays <= cfg.ReservePolicy.WoodResumeDays {
		return WorkPolicyDecision{RedirectWood: true, Reason: "wood reserves need replenishing"}
	}
	if foodDays >= cfg.ReservePolicy.FoodStopDays && woodDays >= cfg.ReservePolicy.WoodStopDays {
		return WorkPolicyDecision{RestAll: true, Reason: "reserves are sufficient; household rests"}
	}
	// When one reserve is already healthy, do not keep producing it merely
	// because the other reserve needs attention. Redirect available labor to
	// the deficient resource and keep the persistent occupations intact.
	if foodDays >= cfg.ReservePolicy.FoodStopDays && woodDays < cfg.ReservePolicy.WoodStopDays {
		return WorkPolicyDecision{RedirectWood: true, Reason: "food reserves are sufficient; prepare wood"}
	}
	if woodDays >= cfg.ReservePolicy.WoodStopDays && foodDays < cfg.ReservePolicy.FoodStopDays {
		return WorkPolicyDecision{RedirectFood: true, Reason: "wood reserves are sufficient; prepare food"}
	}
	return WorkPolicyDecision{}
}

func dailyConsumption(state DailyLaborState, cfg DailyLaborConfig) int64 {
	total := int64(0)
	for _, c := range state.Characters {
		if c.Status == "dead" {
			continue
		}
		age, err := calendar.DailyLaborDefinition.Age(calendar.GameDay(c.BirthGameDay), state.CurrentGameDay)
		if err == nil && age < cfg.ConsumptionRules.ChildAgeYears {
			total += cfg.ConsumptionRules.PerChildPerDayMilli
		} else {
			total += cfg.ConsumptionRules.PerAdultPerDayMilli
		}
	}
	return total
}

func DailyConsumptionPerDay(state DailyLaborState, cfg DailyLaborConfig) int64 {
	return dailyConsumption(state, cfg)
}
