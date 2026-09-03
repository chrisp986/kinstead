package politics

import "fmt"

type DemandType string
type DecisionStatus string
type Option string
type Standing string
type Score int

const (
	DemandLaborService DemandType = "political_labor_service"
	DemandLevy         DemandType = "political_levy"

	OptionServe     Option = "serve"
	OptionPayWood   Option = "pay_wood"
	OptionPaySilver Option = "pay_silver"
	OptionRefuse    Option = "refuse"

	StatusPending      DecisionStatus = "pending"
	StatusResolved     DecisionStatus = "resolved"
	StatusAutoResolved DecisionStatus = "auto_resolved"

	StandingDisapproving Standing = "disapproving"
	StandingNeutral      Standing = "neutral"
	StandingFavorable    Standing = "favorable"
	StandingConnected    Standing = "connected"
)

const (
	LaborServiceTicks    int64 = 4
	LevyWoodMilli        int64 = 18_000
	LevySilverMilli      int64 = 6_000
	StandingHonoredDelta       = 10
	StandingRefusedDelta       = -5
)

var ErrInvalidDemandType = fmt.Errorf("invalid political demand type")
var ErrInvalidOption = fmt.Errorf("invalid political demand option")
var ErrExpired = fmt.Errorf("political demand has expired")

type Resolution struct {
	Option         Option
	StandingDelta  int
	ResourceCode   string
	ResourceMilli  int64
	RequiresWorker bool
	ServiceTicks   int64
}

func AvailableOptions(d DemandType) []Option {
	switch d {
	case DemandLaborService:
		return []Option{OptionServe, OptionRefuse}
	case DemandLevy:
		return []Option{OptionPayWood, OptionPaySilver, OptionRefuse}
	default:
		return nil
	}
}

func ValidateOption(d DemandType, option Option) error {
	for _, allowed := range AvailableOptions(d) {
		if allowed == option {
			return nil
		}
	}
	return fmt.Errorf("%w: %s for %s", ErrInvalidOption, option, d)
}

func ResolveChoice(d DemandType, option Option) (Resolution, error) {
	if err := ValidateOption(d, option); err != nil {
		return Resolution{}, err
	}
	r := Resolution{Option: option}
	if option == OptionRefuse {
		r.StandingDelta = StandingRefusedDelta
		return r, nil
	}
	r.StandingDelta = StandingHonoredDelta
	switch option {
	case OptionServe:
		r.RequiresWorker, r.ServiceTicks = true, LaborServiceTicks
	case OptionPayWood:
		r.ResourceCode, r.ResourceMilli = "wood", LevyWoodMilli
	case OptionPaySilver:
		r.ResourceCode, r.ResourceMilli = "silver", LevySilverMilli
	}
	return r, nil
}

func ResolveExpiry(d DemandType) (Resolution, error) { return ResolveChoice(d, OptionRefuse) }

// ResponseAllowed enforces the exclusive expiry deadline used by production ticks.
func ResponseAllowed(currentTick, expiresTick int64) error {
	if currentTick >= expiresTick {
		return ErrExpired
	}
	return nil
}

func ClampScore(score Score) Score {
	if score < -100 {
		return -100
	}
	if score > 100 {
		return 100
	}
	return score
}

func DeriveStanding(score Score) Standing {
	score = ClampScore(score)
	switch {
	case score <= -31:
		return StandingDisapproving
	case score <= 29:
		return StandingNeutral
	case score <= 69:
		return StandingFavorable
	default:
		return StandingConnected
	}
}
