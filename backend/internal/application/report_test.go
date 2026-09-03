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
