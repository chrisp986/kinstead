package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"game/backend/internal/calendar"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
)

type CalendarEventKind string

var (
	ErrInvalidCalendarCategory = errors.New("invalid calendar category")
	ErrInvalidCalendarRange    = errors.New("invalid calendar range")
	ErrCalendarFromRequired    = errors.New("from_game_day is required when to_game_day is supplied")
)

const (
	CalendarCategorySeason   = "season"
	CalendarCategoryShipment = "shipment"
	CalendarCategoryContract = "contract"
	CalendarCategoryPolitics = "politics"
	CalendarCategoryWorld    = "world"
	CalendarCategoryFarm     = "farm"
)

const (
	CalendarSeasonStart       CalendarEventKind = "season_start"
	CalendarFestival          CalendarEventKind = "festival"
	CalendarHarvest           CalendarEventKind = "harvest"
	CalendarDeliveryDue       CalendarEventKind = "delivery_due"
	CalendarDispatchDeadline  CalendarEventKind = "dispatch_deadline"
	CalendarShipmentArrival   CalendarEventKind = "shipment_arrival"
	CalendarPoliticalDeadline CalendarEventKind = "political_deadline"
	CalendarAssignmentEnd     CalendarEventKind = "assignment_end"
	CalendarAssembly          CalendarEventKind = "assembly"
)

type CalendarEvent struct {
	ID                        string            `json:"id"`
	Kind                      CalendarEventKind `json:"kind"`
	Category                  string            `json:"category"`
	GameDay                   int64             `json:"game_day"`
	EndGameDay                *int64            `json:"end_game_day,omitempty"`
	Importance                string            `json:"importance"`
	ActionRequired            bool              `json:"action_required"`
	RelatedID                 string            `json:"related_id,omitempty"`
	ResourceType              string            `json:"resource_type,omitempty"`
	QuantityMilli             int64             `json:"quantity_milli,omitempty"`
	CounterpartyHouseholdID   string            `json:"counterparty_household_id,omitempty"`
	CounterpartyHouseholdName string            `json:"counterparty_household_name,omitempty"`
	Status                    string            `json:"status,omitempty"`
	Code                      string            `json:"code,omitempty"`
}

type NextHalfYear struct {
	Type      calendar.HalfYear `json:"type"`
	GameDay   int64             `json:"game_day"`
	DaysUntil int64             `json:"days_until"`
}

type CalendarProjection struct {
	HouseholdID    string          `json:"household_id"`
	WorldID        string          `json:"world_id"`
	StartYear      int32           `json:"setting_start_year"`
	CurrentGameDay int64           `json:"current_game_day"`
	Current        calendar.Date   `json:"calendar"`
	NextHalfYear   NextHalfYear    `json:"next_half_year"`
	FromGameDay    int64           `json:"from_game_day"`
	ToGameDay      int64           `json:"to_game_day"`
	Events         []CalendarEvent `json:"events"`
}

type CalendarService struct {
	Reports port.ReportReader
	Reader  port.CalendarReader
}

func NewCalendarService(store port.ReportReader) *CalendarService {
	service := &CalendarService{Reports: store}
	if reader, ok := store.(port.CalendarReader); ok {
		service.Reader = reader
	}
	return service
}

func (s *CalendarService) Household(ctx context.Context, householdID string, from, to int64, category string) (CalendarProjection, error) {
	return s.householdRange(ctx, householdID, &from, &to, category, from == 0 && to == 0)
}

// HouseholdRange preserves query-parameter presence separately from values;
// game day zero is a valid explicit range endpoint.
func (s *CalendarService) HouseholdRange(ctx context.Context, householdID string, from, to *int64, category string) (CalendarProjection, error) {
	return s.householdRange(ctx, householdID, from, to, category, from == nil && to == nil)
}

func (s *CalendarService) householdRange(ctx context.Context, householdID string, fromValue, toValue *int64, category string, useDefault bool) (CalendarProjection, error) {
	snap, err := s.Reports.GetHouseholdReport(ctx, householdID)
	if err != nil {
		return CalendarProjection{}, err
	}
	if !validCalendarCategory(category) {
		return CalendarProjection{}, ErrInvalidCalendarCategory
	}
	var from, to int64
	switch {
	case useDefault:
		from = snap.CurrentGameDay
		var ok bool
		to, ok = addCalendarDays(from, 182)
		if !ok {
			return CalendarProjection{}, fmt.Errorf("%w: range is too large", ErrInvalidCalendarRange)
		}
	case fromValue != nil && toValue == nil:
		from = *fromValue
		var ok bool
		to, ok = addCalendarDays(from, 182)
		if !ok {
			return CalendarProjection{}, fmt.Errorf("%w: range is too large", ErrInvalidCalendarRange)
		}
	case fromValue != nil && toValue != nil:
		from, to = *fromValue, *toValue
	default:
		return CalendarProjection{}, ErrCalendarFromRequired
	}
	if from < 0 || to < from || to-from > 364 {
		return CalendarProjection{}, fmt.Errorf("%w: range must be ordered and no longer than one year", ErrInvalidCalendarRange)
	}
	contextValue := port.CalendarContext{Snapshot: snap}
	if s.Reader != nil {
		contextValue, err = s.Reader.LoadCalendarContext(ctx, householdID, from, to)
		if err != nil {
			return CalendarProjection{}, err
		}
		snap = contextValue.Snapshot
	}
	current := calendar.GameDay(snap.CurrentGameDay)
	events := seasonalEvents(from, to)
	events = append(events, anchorEvents(from, to)...)
	if s.Reader != nil {
		sourcedEvents, err := sourceEvents(contextValue, from, to)
		if err != nil {
			return CalendarProjection{}, err
		}
		events = append(events, sourcedEvents...)
	}
	events = filterCalendarEvents(events, category)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].GameDay != events[j].GameDay {
			return events[i].GameDay < events[j].GameDay
		}
		if calendarEventPriority(events[i].Kind) != calendarEventPriority(events[j].Kind) {
			return calendarEventPriority(events[i].Kind) < calendarEventPriority(events[j].Kind)
		}
		if importanceRank(events[i].Importance) != importanceRank(events[j].Importance) {
			return importanceRank(events[i].Importance) < importanceRank(events[j].Importance)
		}
		return events[i].ID < events[j].ID
	})
	next, ok := nextHalfYearGameDay(current)
	if !ok {
		return CalendarProjection{}, fmt.Errorf("%w: next half-year is outside the supported range", ErrInvalidCalendarRange)
	}
	return CalendarProjection{
		HouseholdID: householdID, WorldID: snap.WorldID, StartYear: snap.SettingStartYear,
		CurrentGameDay: snap.CurrentGameDay, Current: calendar.Breakdown(current),
		NextHalfYear: NextHalfYear{Type: calendar.HalfYearAt(next), GameDay: int64(next), DaysUntil: calendar.DaysUntil(current, next)},
		FromGameDay:  from, ToGameDay: to, Events: events,
	}, nil
}

func seasonalEvents(from, to int64) []CalendarEvent {
	var events []CalendarEvent
	firstYear := calendar.YearIndex(calendar.GameDay(from))
	lastYear := calendar.YearIndex(calendar.GameDay(to))
	starts := []struct {
		season calendar.ProductionSeason
		day    int64
	}{
		{calendar.Spring, 0}, {calendar.Summer, 91},
		{calendar.Autumn, 182}, {calendar.Winter, 273},
	}
	for year := firstYear; year <= lastYear; year++ {
		for _, start := range starts {
			day, ok := recurringGameDay(year, start.day)
			if !ok || day < from || day > to {
				continue
			}
			events = append(events, CalendarEvent{ID: fmt.Sprintf("season-%d", day), Kind: CalendarSeasonStart, Category: CalendarCategorySeason, GameDay: day, Importance: "important", Code: string(start.season)})
		}
	}
	return events
}

func anchorEvents(from, to int64) []CalendarEvent {
	var events []CalendarEvent
	firstYear := calendar.YearIndex(calendar.GameDay(from))
	lastYear := calendar.YearIndex(calendar.GameDay(to))
	for _, rule := range calendar.DefaultAnchors() {
		for year := firstYear; year <= lastYear; year++ {
			day, ok := recurringGameDay(year, rule.DayOfYear)
			if !ok || day < from || day > to {
				continue
			}
			kind := CalendarEventKind(rule.Kind)
			category, importance := CalendarCategoryWorld, "context"
			if rule.Kind == calendar.AnchorSeasonStart {
				continue
			} else if rule.Kind == calendar.AnchorHarvest {
				category, importance, kind = CalendarCategoryFarm, "important", CalendarHarvest
			} else if rule.Kind == calendar.AnchorAssembly {
				category, kind = CalendarCategoryWorld, CalendarAssembly
			}
			events = append(events, CalendarEvent{ID: fmt.Sprintf("anchor-%s-%d", rule.Code, year), Kind: kind, Category: category, GameDay: day, Importance: importance, Code: rule.Code})
		}
	}
	return events
}

func sourceEvents(value port.CalendarContext, from, to int64) ([]CalendarEvent, error) {
	var events []CalendarEvent
	for _, item := range value.Obligations {
		if item.DueGameDay >= from && item.DueGameDay <= to {
			events = append(events, CalendarEvent{ID: "obligation-" + item.ID, Kind: CalendarDeliveryDue, Category: CalendarCategoryContract, GameDay: item.DueGameDay, Importance: "important", RelatedID: item.ID, ResourceType: item.ResourceType, QuantityMilli: item.QuantityMilli, CounterpartyHouseholdID: oppositeHousehold(value.Snapshot.HouseholdID, item.DebtorHouseholdID, item.CreditorHouseholdID), CounterpartyHouseholdName: item.CounterpartyName, Status: item.Status})
		}
		if item.ShipmentID == "" && item.DebtorHouseholdID == value.Snapshot.HouseholdID {
			deadline := item.LatestDispatchGameDay
			if deadline >= from && deadline <= to {
				events = append(events, CalendarEvent{ID: "dispatch-" + item.ID, Kind: CalendarDispatchDeadline, Category: CalendarCategoryContract, GameDay: deadline, Importance: "critical", ActionRequired: true, RelatedID: item.ID, ResourceType: item.ResourceType, QuantityMilli: item.QuantityMilli, CounterpartyHouseholdName: item.CounterpartyName, Status: item.Status})
			}
		}
	}
	for _, item := range value.Shipments {
		switch item.Status {
		case string(shipmentdomain.StatusArrived):
			if item.ActualArrivalGameDay != nil && *item.ActualArrivalGameDay >= from && *item.ActualArrivalGameDay <= to {
				events = append(events, CalendarEvent{ID: "shipment-arrived-" + item.ID, Kind: CalendarShipmentArrival, Category: CalendarCategoryShipment, GameDay: *item.ActualArrivalGameDay, Importance: "important", RelatedID: item.ID, ResourceType: item.ResourceType, QuantityMilli: item.QuantityMilli, CounterpartyHouseholdName: item.CounterpartyName, Status: item.Status})
			}
		case string(shipmentdomain.StatusInTransit):
			if item.ExpectedArrivalGameDay >= from && item.ExpectedArrivalGameDay <= to {
				events = append(events, CalendarEvent{ID: "shipment-" + item.ID, Kind: CalendarShipmentArrival, Category: CalendarCategoryShipment, GameDay: item.ExpectedArrivalGameDay, Importance: "important", RelatedID: item.ID, ResourceType: item.ResourceType, QuantityMilli: item.QuantityMilli, CounterpartyHouseholdName: item.CounterpartyName, Status: item.Status})
			}
		}
	}
	for _, item := range value.Deadlines {
		day := item.DeadlineGameDay
		if day >= from && day <= to {
			events = append(events, CalendarEvent{ID: item.ID, Kind: CalendarEventKind(item.Kind), Category: item.Category, GameDay: day, Importance: item.Importance, ActionRequired: true, RelatedID: item.ID})
		}
	}
	for _, item := range value.Assignments {
		day, err := tickGameDay(value.Snapshot, item.EndsTick)
		if err != nil {
			return nil, fmt.Errorf("project assignment end game day: %w", err)
		}
		if day >= from && day <= to {
			events = append(events, CalendarEvent{ID: "assignment-" + item.ID, Kind: CalendarAssignmentEnd, Category: CalendarCategoryFarm, GameDay: day, Importance: item.Importance, RelatedID: item.ID})
		}
	}
	return events, nil
}

func oppositeHousehold(current, debtor, creditor string) string {
	if current == debtor {
		return creditor
	}
	return debtor
}

func filterCalendarEvents(events []CalendarEvent, category string) []CalendarEvent {
	if category == "" || category == "all" {
		return events
	}
	filtered := events[:0]
	for _, event := range events {
		if event.Category == category {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func validCalendarCategory(category string) bool {
	switch category {
	case "", "all", CalendarCategorySeason, CalendarCategoryShipment, CalendarCategoryContract,
		CalendarCategoryPolitics, CalendarCategoryWorld, CalendarCategoryFarm:
		return true
	default:
		return false
	}
}

func importanceRank(value string) int {
	switch value {
	case "critical":
		return 0
	case "important":
		return 1
	default:
		return 2
	}
}

func tickGameDay(snap port.HouseholdSnapshot, tick int64) (int64, error) {
	tickOffset, err := calendar.SubtractInt64(tick, snap.CurrentTick)
	if err != nil {
		return 0, err
	}
	day, err := calendar.GameDayAtTick(calendar.GameDay(snap.CurrentGameDay), snap.CalendarRemainder,
		snap.GameDaysPerTickNum, snap.GameDaysPerTickDen, tickOffset)
	if err != nil {
		return 0, err
	}
	return int64(day), nil
}

func calendarEventPriority(kind CalendarEventKind) int {
	switch kind {
	case CalendarPoliticalDeadline:
		return 0
	case CalendarDeliveryDue, CalendarDispatchDeadline:
		return 1
	case CalendarShipmentArrival:
		return 2
	case CalendarFestival, CalendarHarvest, CalendarAssembly:
		return 3
	case CalendarSeasonStart:
		return 4
	default:
		return 5
	}
}

func addCalendarDays(value, days int64) (int64, bool) {
	if days > 0 && value > math.MaxInt64-days {
		return 0, false
	}
	if days < 0 && value < math.MinInt64-days {
		return 0, false
	}
	return value + days, true
}

func recurringGameDay(year, dayOfYear int64) (int64, bool) {
	if year < 0 || dayOfYear < 0 || dayOfYear >= calendar.DaysPerYear {
		return 0, false
	}
	if year > (math.MaxInt64-dayOfYear)/calendar.DaysPerYear {
		return 0, false
	}
	return year*calendar.DaysPerYear + dayOfYear, true
}

func nextHalfYearGameDay(day calendar.GameDay) (calendar.GameDay, bool) {
	base, ok := recurringGameDay(calendar.YearIndex(day), 0)
	if !ok {
		return 0, false
	}
	offset := int64(182)
	if calendar.DayOfYear(day) >= 182 {
		offset = calendar.DaysPerYear
	}
	next, ok := addCalendarDays(base, offset)
	return calendar.GameDay(next), ok
}
