package report

// Item is a structured Farm Report fact. The backend never stores localized
// prose in report items; clients render these stable codes and data fields.
type Item struct {
	Code       string         `json:"code"`
	Severity   string         `json:"severity"`
	Target     string         `json:"target,omitempty"`
	RelatedID  string         `json:"related_id,omitempty"`
	DueTick    *int64         `json:"-"`
	DueGameDay *int64         `json:"due_game_day,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

type Character struct {
	ID      string
	Name    string
	Fatigue int
}

type PoliticalDemand struct {
	ID             string
	ActorName      string
	ExpiresTick    int64
	ExpiresGameDay int64
}

type ContractObligation struct {
	ID                     string
	ResourceType           string
	QuantityMilli          int64
	DueArrivalTick         int64
	ExpectedArrivalTick    *int64
	DueGameDay             int64
	ExpectedArrivalGameDay *int64
}

type Input struct {
	CurrentTick         int64
	CurrentGameDay      int64
	SupplyDays          float64
	Characters          []Character
	PoliticalDemands    []PoliticalDemand
	ContractObligations []ContractObligation
}
