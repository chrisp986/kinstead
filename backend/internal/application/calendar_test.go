package application

import (
	"context"
	"testing"

	"game/backend/internal/port"
)

type calendarReaderStub struct {
	snapshot port.HouseholdSnapshot
	value    port.CalendarContext
}

func (s calendarReaderStub) GetHouseholdReport(context.Context, string) (port.HouseholdSnapshot, error) {
	return s.snapshot, nil
}

func (s calendarReaderStub) LoadCalendarContext(context.Context, string, int64, int64) (port.CalendarContext, error) {
	return s.value, nil
}

func TestCalendarProjectionIncludesAnchorsAndHouseholdEvents(t *testing.T) {
	snapshot := port.HouseholdSnapshot{
		HouseholdID: "household", WorldID: "world", CurrentGameDay: 0,
		GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
	}
	reader := calendarReaderStub{
		snapshot: snapshot,
		value: port.CalendarContext{
			Snapshot: snapshot,
			Obligations: []port.CalendarObligationRecord{{
				ID: "obligation", DebtorHouseholdID: "household", CreditorHouseholdID: "other",
				ResourceType: "provisions", QuantityMilli: 1000, DueGameDay: 91,
				LatestDispatchGameDay: 84, Status: "pending",
			}},
		},
	}
	projection, err := NewCalendarService(reader).Household(context.Background(), "household", 0, 182, "")
	if err != nil {
		t.Fatal(err)
	}
	if projection.NextHalfYear.GameDay != 182 || projection.NextHalfYear.DaysUntil != 182 {
		t.Fatalf("next half-year = %+v", projection.NextHalfYear)
	}
	seen := map[CalendarEventKind]bool{}
	count := 0
	for _, event := range projection.Events {
		if event.Kind == CalendarSeasonStart && event.GameDay == 91 {
			count++
		}
		seen[event.Kind] = true
	}
	if count != 1 {
		t.Fatalf("summer start event count = %d, want one", count)
	}
	if !seen[CalendarFestival] || !seen[CalendarDeliveryDue] || !seen[CalendarDispatchDeadline] {
		t.Fatalf("projection kinds = %v", seen)
	}

	trade, err := NewCalendarService(reader).Household(context.Background(), "household", 0, 182, "trade")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range trade.Events {
		if event.Category != "trade" {
			t.Fatalf("trade filter returned %q event", event.Category)
		}
	}
}
