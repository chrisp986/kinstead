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

// DefaultAnchorsFor returns anchors under an explicit calendar definition.
// The one-day autumn/winter shift in the daily model is reflected here so
// calendar projections and simulation agree at season boundaries.
func DefaultAnchorsFor(def CalendarDefinition) []AnchorRule {
	anchors := DefaultAnchors()
	if def.DaysPerYear == DailyLaborDefinition.DaysPerYear {
		for i := range anchors {
			switch anchors[i].Code {
			case "harvest_start":
				anchors[i].DayOfYear = 152
			case "thing":
				anchors[i].DayOfYear = 288
			case "winter_start":
				anchors[i].DayOfYear = 274
			case "midwinter":
				anchors[i].DayOfYear = 305
			case "jol":
				anchors[i].DayOfYear = 321
			}
		}
	}
	return anchors
}

func AnchorGameDay(rule AnchorRule, year int64) GameDay {
	return GameDay(year*DaysPerYear + rule.DayOfYear)
}
