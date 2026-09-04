package application

import (
	"context"
	"errors"
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

	shipment, err := NewCalendarService(reader).Household(context.Background(), "household", 0, 182, "shipment")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range shipment.Events {
		if event.Category != "shipment" {
			t.Fatalf("shipment filter returned %q event", event.Category)
		}
	}
}

func TestCalendarRangePresenceAndValidation(t *testing.T) {
	snapshot := port.HouseholdSnapshot{HouseholdID: "household", WorldID: "world", CurrentGameDay: 45, SettingStartYear: 980}
	reader := calendarReaderStub{snapshot: snapshot, value: port.CalendarContext{Snapshot: snapshot}}
	service := NewCalendarService(reader)
	zero := int64(0)
	projection, err := service.HouseholdRange(context.Background(), "household", &zero, &zero, "season")
	if err != nil {
		t.Fatal(err)
	}
	if projection.FromGameDay != 0 || projection.ToGameDay != 0 {
		t.Fatalf("explicit zero range = %d..%d", projection.FromGameDay, projection.ToGameDay)
	}
	from := int64(45)
	to := int64(100)
	projection, err = service.HouseholdRange(context.Background(), "household", &from, &to, "season")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range projection.Events {
		if event.GameDay == 45 {
			t.Fatal("range start created a false season event")
		}
	}
	if len(projection.Events) != 1 || projection.Events[0].GameDay != 91 || projection.Events[0].Kind != CalendarSeasonStart {
		t.Fatalf("season events for 45..100 = %+v", projection.Events)
	}
	if _, err := service.HouseholdRange(context.Background(), "household", nil, &to, "season"); err == nil {
		t.Fatal("to-only calendar range was accepted")
	}
	if _, err := service.HouseholdRange(context.Background(), "household", &zero, &to, "unknown"); !errors.Is(err, ErrInvalidCalendarCategory) {
		t.Fatalf("unknown category error = %v", err)
	}
	tooFar := int64(365)
	if _, err := service.HouseholdRange(context.Background(), "household", &zero, &tooFar, "season"); err == nil {
		t.Fatal("365-day calendar range was accepted")
	}
}
