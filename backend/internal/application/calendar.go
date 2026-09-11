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

type NextSeason struct {
	Type      calendar.ProductionSeason `json:"type"`
	GameDay   int64                     `json:"game_day"`
	DaysUntil int64                     `json:"days_until"`
}

type CalendarProjection struct {
	HouseholdID    string          `json:"household_id"`
	WorldID        string          `json:"world_id"`
	StartYear      int32           `json:"setting_start_year"`
	CurrentGameDay int64           `json:"current_game_day"`
	Current        calendar.Date   `json:"calendar"`
	NextHalfYear   *NextHalfYear   `json:"next_half_year,omitempty"`
	NextSeason     *NextSeason     `json:"next_season,omitempty"`
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
	definition, err := calendar.DefinitionForModel(string(snap.SimulationModel))
	if err != nil {
		return CalendarProjection{}, err
	}
	monthlySpan := int64(0)
	if snap.SimulationModel == port.ModelMonthlySeasons {
		moment, momentErr := calendar.MomentAtClock(calendar.ClockState{Day: calendar.GameDay(snap.CurrentGameDay), Remainder: snap.CalendarRemainder, GameDaysPerTickNum: snap.GameDaysPerTickNum, GameDaysPerTickDen: snap.GameDaysPerTickDen})
		if momentErr != nil {
			return CalendarProjection{}, momentErr
		}
		temporal, temporalErr := temporalContext(snap.SimulationModel, snap.CalendarAnchorAt, snap.WorldUTCOffsetMinutes, moment)
		if temporalErr != nil {
			return CalendarProjection{}, temporalErr
		}
		monthlySpan = int64(temporal.Position.LengthDays - temporal.Position.Day + 1)
	}
	if !validCalendarCategory(category) {
		return CalendarProjection{}, ErrInvalidCalendarCategory
	}
	var from, to int64
	switch {
	case useDefault:
		from = snap.CurrentGameDay
		span := definition.HalfYearStart
		if snap.SimulationModel == port.ModelMonthlySeasons {
			span = monthlySpan
		}
		var ok bool
		to, ok = addCalendarDays(from, span)
		if !ok {
			return CalendarProjection{}, fmt.Errorf("%w: range is too large", ErrInvalidCalendarRange)
		}
	case fromValue != nil && toValue == nil:
		from = *fromValue
		span := definition.HalfYearStart
		if snap.SimulationModel == port.ModelMonthlySeasons {
			span = monthlySpan
		}
		var ok bool
		to, ok = addCalendarDays(from, span)
		if !ok {
			return CalendarProjection{}, fmt.Errorf("%w: range is too large", ErrInvalidCalendarRange)
		}
	case fromValue != nil && toValue != nil:
		from, to = *fromValue, *toValue
	default:
		return CalendarProjection{}, ErrCalendarFromRequired
	}
	maxRange := definition.DaysPerYear
	if snap.SimulationModel == port.ModelMonthlySeasons {
		maxRange = 366
	}
	if from < 0 || to < from || to-from > maxRange {
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
	events := seasonalEvents(from, to, definition)
	if snap.SimulationModel == port.ModelMonthlySeasons {
		events, err = monthlySeasonalEvents(snap, from, to)
		if err != nil {
			return CalendarProjection{}, err
		}
	} else {
		events = append(events, anchorEvents(from, to, definition)...)
	}
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
	var nextHalf *NextHalfYear
	var nextSeason *NextSeason
	breakdown := definition.Breakdown(current)
	if snap.SimulationModel == port.ModelMonthlySeasons {
		moment, _ := calendar.MomentAtClock(calendar.ClockState{Day: current, Remainder: snap.CalendarRemainder, GameDaysPerTickNum: snap.GameDaysPerTickNum, GameDaysPerTickDen: snap.GameDaysPerTickDen})
		temporal, temporalErr := temporalContext(snap.SimulationModel, snap.CalendarAnchorAt, snap.WorldUTCOffsetMinutes, moment)
		if temporalErr != nil {
			return CalendarProjection{}, temporalErr
		}
		days := int64(temporal.Position.LengthDays - temporal.Position.Day + 1)
		boundaryDay, valid := addCalendarDays(snap.CurrentGameDay, days)
		if !valid {
			return CalendarProjection{}, ErrInvalidCalendarRange
		}
		nextType, _ := calendar.SeasonForMonth(temporal.Position.NextBoundary.Month())
		nextSeason = &NextSeason{Type: nextType, GameDay: boundaryDay, DaysUntil: days}
		breakdown.ProductionSeason = temporal.Position.Season
	} else {
		next, valid := nextHalfYearGameDay(current, definition)
		if !valid {
			return CalendarProjection{}, fmt.Errorf("%w: next half-year is outside the supported range", ErrInvalidCalendarRange)
		}
		value := NextHalfYear{Type: definition.HalfYearAt(next), GameDay: int64(next), DaysUntil: calendar.DaysUntil(current, next)}
		nextHalf = &value
	}
	return CalendarProjection{
		HouseholdID: householdID, WorldID: snap.WorldID, StartYear: snap.SettingStartYear,
		CurrentGameDay: snap.CurrentGameDay, Current: breakdown,
		NextHalfYear: nextHalf, NextSeason: nextSeason,
		FromGameDay: from, ToGameDay: to, Events: events,
	}, nil
}

func monthlySeasonalEvents(snap port.HouseholdSnapshot, from, to int64) ([]CalendarEvent, error) {
	if snap.CalendarAnchorAt == nil || snap.WorldUTCOffsetMinutes == nil {
		return nil, ErrUnsupportedSimulationModel
	}
	events := make([]CalendarEvent, 0, 12)
	for day := from; day <= to; day++ {
		date, err := calendar.SchedulingDate(*snap.CalendarAnchorAt, *snap.WorldUTCOffsetMinutes, calendar.Moment{Day: calendar.GameDay(day), Hour: 0})
		if err != nil {
			return nil, err
		}
		if date.Day() != 1 {
			continue
		}
		season, err := calendar.SeasonForMonth(date.Month())
		if err != nil {
			return nil, err
		}
		events = append(events, CalendarEvent{ID: fmt.Sprintf("season-%d", day), Kind: CalendarSeasonStart, Category: CalendarCategorySeason, GameDay: day, Importance: "important", Code: string(season)})
		if day == math.MaxInt64 {
			break
		}
	}
	return events, nil
}

func seasonalEvents(from, to int64, definition calendar.CalendarDefinition) []CalendarEvent {
	var events []CalendarEvent
	firstYear := definition.YearIndex(calendar.GameDay(from))
	lastYear := definition.YearIndex(calendar.GameDay(to))
	starts := []struct {
		season calendar.ProductionSeason
		day    int64
	}{
		{calendar.Spring, 0}, {calendar.Summer, definition.SpringEnd},
		{calendar.Autumn, definition.SummerEnd}, {calendar.Winter, definition.AutumnEnd},
	}
	for year := firstYear; year <= lastYear; year++ {
		for _, start := range starts {
			day, ok := recurringGameDay(year, start.day, definition)
			if !ok || day < from || day > to {
				continue
			}
			events = append(events, CalendarEvent{ID: fmt.Sprintf("season-%d", day), Kind: CalendarSeasonStart, Category: CalendarCategorySeason, GameDay: day, Importance: "important", Code: string(start.season)})
		}
	}
	return events
}

func anchorEvents(from, to int64, definition calendar.CalendarDefinition) []CalendarEvent {
	var events []CalendarEvent
	firstYear := definition.YearIndex(calendar.GameDay(from))
	lastYear := definition.YearIndex(calendar.GameDay(to))
	for _, rule := range calendar.DefaultAnchorsFor(definition) {
		for year := firstYear; year <= lastYear; year++ {
			day, ok := recurringGameDay(year, rule.DayOfYear, definition)
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

func recurringGameDay(year, dayOfYear int64, definition calendar.CalendarDefinition) (int64, bool) {
	if year < 0 || dayOfYear < 0 || dayOfYear >= definition.DaysPerYear {
		return 0, false
	}
	if year > (math.MaxInt64-dayOfYear)/definition.DaysPerYear {
		return 0, false
	}
	return year*definition.DaysPerYear + dayOfYear, true
}

func nextHalfYearGameDay(day calendar.GameDay, definition calendar.CalendarDefinition) (calendar.GameDay, bool) {
	base, ok := recurringGameDay(definition.YearIndex(day), 0, definition)
	if !ok {
		return 0, false
	}
	offset := definition.HalfYearStart
	if definition.DayOfYear(day) >= definition.HalfYearStart {
		offset = definition.DaysPerYear
	}
	next, ok := addCalendarDays(base, offset)
	return calendar.GameDay(next), ok
}
