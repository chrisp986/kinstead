package application

import (
	"context"
	"errors"
	"math"
	"testing"

	"game/backend/internal/calendar"
	shipmentdomain "game/backend/internal/domain/shipment"
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
	for _, event := range projection.Events {
		if event.Kind == CalendarSeasonStart && event.GameDay == 91 && event.Code != "summer" {
			t.Fatalf("summer season event code = %q", event.Code)
		}
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
	max := int64(math.MaxInt64)
	if _, err := service.HouseholdRange(context.Background(), "household", &max, nil, "season"); !errors.Is(err, ErrInvalidCalendarRange) {
		t.Fatalf("overflowing open-ended calendar range error = %v", err)
	}
	overflowingSnapshot := snapshot
	overflowingSnapshot.CurrentGameDay = max
	if _, err := NewCalendarService(calendarReaderStub{snapshot: overflowingSnapshot}).HouseholdRange(context.Background(), "household", nil, nil, "season"); !errors.Is(err, ErrInvalidCalendarRange) {
		t.Fatalf("overflowing default calendar range error = %v", err)
	}
	if _, err := NewCalendarService(calendarReaderStub{snapshot: overflowingSnapshot, value: port.CalendarContext{Snapshot: overflowingSnapshot}}).HouseholdRange(context.Background(), "household", &zero, &zero, "season"); !errors.Is(err, ErrInvalidCalendarRange) {
		t.Fatalf("overflowing next half-year error = %v", err)
	}
}

func TestCalendarShipmentProjectionExcludesInactiveStatuses(t *testing.T) {
	snapshot := port.HouseholdSnapshot{
		HouseholdID: "household", WorldID: "world", CurrentGameDay: 0,
		GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
	}
	actual := int64(12)
	reader := calendarReaderStub{
		snapshot: snapshot,
		value: port.CalendarContext{
			Snapshot: snapshot,
			Shipments: []port.CalendarShipmentRecord{
				{ID: "in-transit", ExpectedArrivalGameDay: 15, Status: string(shipmentdomain.StatusInTransit)},
				{ID: "arrived", ActualArrivalGameDay: &actual, Status: string(shipmentdomain.StatusArrived)},
				{ID: "cancelled", ExpectedArrivalGameDay: 15, Status: string(shipmentdomain.StatusCancelled)},
				{ID: "prepared", ExpectedArrivalGameDay: 15, Status: string(shipmentdomain.StatusPrepared)},
			},
		},
	}
	projection, err := NewCalendarService(reader).HouseholdRange(context.Background(), "household", ptr(int64(0)), ptr(int64(30)), CalendarCategoryShipment)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Events) != 2 {
		t.Fatalf("shipment events = %+v, want only in-transit and arrived", projection.Events)
	}
	seen := map[string]string{}
	for _, event := range projection.Events {
		seen[event.RelatedID] = event.Status
	}
	if _, ok := seen["in-transit"]; !ok {
		t.Fatal("in-transit shipment was not projected")
	}
	if _, ok := seen["arrived"]; !ok {
		t.Fatal("arrived shipment was not projected")
	}
	if _, ok := seen["cancelled"]; ok {
		t.Fatal("cancelled shipment was projected")
	}
	if _, ok := seen["prepared"]; ok {
		t.Fatal("prepared shipment was projected")
	}
}

func TestCalendarProjectionPropagatesAssignmentArithmeticErrors(t *testing.T) {
	snapshot := port.HouseholdSnapshot{
		HouseholdID: "household", WorldID: "world", CurrentGameDay: 0,
		CurrentTick: math.MinInt64, GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
	}
	reader := calendarReaderStub{
		snapshot: snapshot,
		value: port.CalendarContext{
			Snapshot:    snapshot,
			Assignments: []port.CalendarAssignmentRecord{{ID: "assignment", EndsTick: math.MaxInt64}},
		},
	}
	_, err := NewCalendarService(reader).HouseholdRange(context.Background(), "household", ptr(int64(0)), ptr(int64(0)), CalendarCategoryFarm)
	if !errors.Is(err, calendar.ErrArithmeticOverflow) {
		t.Fatalf("assignment arithmetic error = %v, want overflow", err)
	}
}

func ptr(value int64) *int64 { return &value }
