package calendar

// AnchorKind identifies a recurring cultural or seasonal marker. These are
// intentionally game-calendar abstractions, not claims about Gregorian dates.
type AnchorKind string

const (
	AnchorSeasonStart AnchorKind = "season_start"
	AnchorFestival    AnchorKind = "festival"
	AnchorHarvest     AnchorKind = "harvest"
	AnchorAssembly    AnchorKind = "assembly"
)

type AnchorRule struct {
	Code      string
	Kind      AnchorKind
	DayOfYear int64
	Recurring bool
}

var defaultAnchorRules = []AnchorRule{
	{Code: "summer_start", Kind: AnchorSeasonStart, DayOfYear: 91, Recurring: true},
	{Code: "midsummer", Kind: AnchorFestival, DayOfYear: 121, Recurring: true},
	{Code: "harvest_start", Kind: AnchorHarvest, DayOfYear: 152, Recurring: true},
	{Code: "thing", Kind: AnchorAssembly, DayOfYear: 287, Recurring: true},
	{Code: "winter_start", Kind: AnchorSeasonStart, DayOfYear: 273, Recurring: true},
	{Code: "midwinter", Kind: AnchorFestival, DayOfYear: 304, Recurring: true},
	{Code: "jol", Kind: AnchorFestival, DayOfYear: 320, Recurring: true},
}

// DefaultAnchors returns a copy so callers cannot mutate the shared v0.3
// configuration. A future world-specific configuration can replace this
// small rule set without changing the event projection model.
func DefaultAnchors() []AnchorRule {
	return append([]AnchorRule(nil), defaultAnchorRules...)
}

func AnchorGameDay(rule AnchorRule, year int64) GameDay {
	return GameDay(year*DaysPerYear + rule.DayOfYear)
}
