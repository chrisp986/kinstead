package v03

func NewBjornvikState() HouseholdState {
	return HouseholdState{
		FarmSpecialization: Fishing,
		ProvisionsMilli:    150_000,
		WoodMilli:          20_000,
		TradeGoodsMilli:    4_000,
		SilverMilli:        30_000,
		Characters: []Character{
			{ID: "bjorn", Name: "Bjorn", LaborPermille: 1000, Specialization: Agriculture},
			{ID: "astrid", Name: "Astrid", LaborPermille: 1000, Specialization: Fishing},
			{ID: "einar", Name: "Einar", LaborPermille: 1000},
			{ID: "ragnhild", Name: "Ragnhild", LaborPermille: 500},
			{ID: "sven", Name: "Sven", LaborPermille: 0},
		},
		Buildings: []BuildingState{
			{Name: "storage", WoodCostMilli: 30_000, WorkerDaysPermille: 6_000},
			{Name: "workshop", WoodCostMilli: 40_000, WorkerDaysPermille: 8_000},
			{Name: "housing", WoodCostMilli: 50_000, WorkerDaysPermille: 10_000},
		},
	}
}

// NewExampleState remains as a compatibility alias for the first scaffold.
func NewExampleState() HouseholdState { return NewBjornvikState() }
