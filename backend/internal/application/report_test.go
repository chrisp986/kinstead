package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"game/backend/internal/balance"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

type reportReaderStub struct{ snapshot port.HouseholdSnapshot }

func (s reportReaderStub) GetHouseholdReport(context.Context, string) (port.HouseholdSnapshot, error) {
	return s.snapshot, nil
}

type farmReportReaderStub struct {
	reportReaderStub
	entries []port.ChronicleEntryRecord
}

func (s farmReportReaderStub) ListRecentChronicleForReport(context.Context, string, int64, int) ([]port.ChronicleEntryRecord, error) {
	return append([]port.ChronicleEntryRecord(nil), s.entries...), nil
}
func (s farmReportReaderStub) ListPendingPoliticalDemandsForReport(context.Context, string) ([]port.PoliticalReportDemand, error) {
	return []port.PoliticalReportDemand{{ID: "demand", ActorName: "Jarl", ExpiresTick: 11}}, nil
}
func (s farmReportReaderStub) ListContractObligationsForReport(context.Context, string) ([]port.ContractReportObligation, error) {
	return []port.ContractReportObligation{{ID: "obligation", ResourceType: "wood", QuantityMilli: 10000, DueArrivalTick: 12}}, nil
}

func TestFarmReportSerializesEmptyCollectionsAsArrays(t *testing.T) {
	reader := reportReaderStub{snapshot: port.HouseholdSnapshot{
		HouseholdID: "household", HouseholdName: "Empty household", WorldID: "world",
		HistoricalStart:          time.Date(980, time.January, 1, 0, 0, 0, 0, time.UTC),
		HistoricalDaysPerTickNum: 365, HistoricalDaysPerTickDen: 48,
	}}
	report, err := (&ReportService{Store: reader, Balance: balance.V03()}).FarmReport(context.Background(), "household")
	if err != nil {
		t.Fatal(err)
	}
	if report.Characters == nil || report.Assignments == nil {
		t.Fatalf("empty report collections must be non-nil: characters=%v assignments=%v", report.Characters, report.Assignments)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Characters  []port.CharacterRecord  `json:"characters"`
		Assignments []port.AssignmentRecord `json:"assignments"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatal(err)
	}
	if body.Characters == nil || body.Assignments == nil {
		t.Fatalf("serialized empty collections must be arrays: %s", payload)
	}
}

func TestFarmReportDerivesHistoricalDateSeasonAndAge(t *testing.T) {
	reader := reportReaderStub{snapshot: port.HouseholdSnapshot{
		HouseholdID: "household", HouseholdName: "Household", WorldID: "world",
		CurrentTick: 48, HistoricalStart: time.Date(980, time.January, 1, 0, 0, 0, 0, time.UTC),
		HistoricalDaysPerTickNum: 365, HistoricalDaysPerTickDen: 48,
		State:      simulation.HouseholdState{ProvisionsMilli: 150_000},
		Characters: []port.CharacterRecord{{ID: "character", Name: "Bjorn", BirthDate: "0948-01-01"}},
	}}
	service := &ReportService{Store: reader, Balance: balance.V03()}
	report, err := service.FarmReport(context.Background(), "household")
	if err != nil {
		t.Fatal(err)
	}
	if report.HistoricalDate != "0980-12-31" || report.Season != simulation.Winter {
		t.Fatalf("date/season = %s/%s", report.HistoricalDate, report.Season)
	}
	if got := report.Characters[0].Age; got != 32 {
		t.Fatalf("age = %d, want 32", got)
	}
}

func TestFarmReportSelectsSignificantRecentChangesAndDecisions(t *testing.T) {
	reader := farmReportReaderStub{reportReaderStub: reportReaderStub{snapshot: port.HouseholdSnapshot{
		HouseholdID: "household", HouseholdName: "Household", WorldID: "world", CurrentTick: 18,
		HistoricalStart: time.Date(980, time.January, 1, 0, 0, 0, 0, time.UTC), HistoricalDaysPerTickNum: 365, HistoricalDaysPerTickDen: 48,
		TickDurationSeconds: 3600, State: simulation.HouseholdState{ProvisionsMilli: 6500},
	}}, entries: []port.ChronicleEntryRecord{{ID: "routine", EntryType: "assignment_completed", OccurredTick: 18}, {ID: "arrival", EntryType: "shipment_arrived", OccurredTick: 17}, {ID: "late", EntryType: "contract_obligation_late", OccurredTick: 16}, {ID: "purchase", EntryType: "market_purchase", OccurredTick: 15}}}
	report, err := (&ReportService{Store: reader, Balance: balance.V03()}).FarmReport(context.Background(), "household")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.RecentChanges) != 3 || report.RecentChanges[0].EntryType != "shipment_arrived" || report.RecentChanges[1].EntryType != "contract_obligation_late" {
		t.Fatalf("recent changes = %+v", report.RecentChanges)
	}
	if len(report.Decisions) != 3 || report.Decisions[0].Code != "secure_provisions" || report.Decisions[1].Code != "respond_political_demand" {
		t.Fatalf("decisions = %+v", report.Decisions)
	}
}
