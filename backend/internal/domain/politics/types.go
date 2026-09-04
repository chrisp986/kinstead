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
var ErrMissingCharacter = fmt.Errorf("political labor service requires a character")
var ErrIneligibleCharacter = fmt.Errorf("character is not eligible for political service")
var ErrServiceOverlap = fmt.Errorf("political service assignment overlaps existing work")
var ErrInsufficientResources = fmt.Errorf("insufficient resources for political demand")

type Resolution struct {
	Option         Option
	StandingDelta  int
	ResourceCode   string
	ResourceMilli  int64
	RequiresWorker bool
	ServiceTicks   int64
}

// DemandTerms are the balance terms snapshotted on a household decision. They
// are part of the domain so an old pending demand is never reinterpreted by a
// later balance change.
type DemandTerms struct {
	ServiceTicks         int64 `json:"service_ticks"`
	WoodCostMilli        int64 `json:"wood_cost_milli"`
	SilverCostMilli      int64 `json:"silver_cost_milli"`
	HonoredStandingDelta int   `json:"honor_standing_delta"`
	RefusedStandingDelta int   `json:"refuse_standing_delta"`
}

func DefaultTerms(d DemandType) DemandTerms {
	t := DemandTerms{HonoredStandingDelta: StandingHonoredDelta, RefusedStandingDelta: StandingRefusedDelta}
	if d == DemandLaborService {
		t.ServiceTicks = LaborServiceTicks
	}
	if d == DemandLevy {
		t.WoodCostMilli, t.SilverCostMilli = LevyWoodMilli, LevySilverMilli
	}
	return t
}

func (t DemandTerms) Validate(d DemandType) error {
	if len(AvailableOptions(d)) == 0 {
		return ErrInvalidDemandType
	}
	if t.HonoredStandingDelta == 0 || t.RefusedStandingDelta == 0 {
		return fmt.Errorf("invalid political demand standing terms")
	}
	switch d {
	case DemandLaborService:
		if t.ServiceTicks <= 0 {
			return fmt.Errorf("invalid political service duration")
		}
	case DemandLevy:
		if t.WoodCostMilli <= 0 || t.SilverCostMilli <= 0 {
			return fmt.Errorf("invalid political levy costs")
		}
	}
	return nil
}

func ResolveChoiceWithTerms(d DemandType, option Option, terms DemandTerms) (Resolution, error) {
	if err := terms.Validate(d); err != nil {
		return Resolution{}, err
	}
	if err := ValidateOption(d, option); err != nil {
		return Resolution{}, err
	}
	r := Resolution{Option: option}
	if option == OptionRefuse {
		r.StandingDelta = terms.RefusedStandingDelta
		return r, nil
	}
	r.StandingDelta = terms.HonoredStandingDelta
	switch option {
	case OptionServe:
		r.RequiresWorker, r.ServiceTicks = true, terms.ServiceTicks
	case OptionPayWood:
		r.ResourceCode, r.ResourceMilli = "wood", terms.WoodCostMilli
	case OptionPaySilver:
		r.ResourceCode, r.ResourceMilli = "silver", terms.SilverCostMilli
	}
	return r, nil
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
	return ResolveChoiceWithTerms(d, option, DefaultTerms(d))
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
