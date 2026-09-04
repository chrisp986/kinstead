package application

import (
	"context"
	"fmt"
	"sort"

	"game/backend/internal/calendar"
	"game/backend/internal/port"
)

type CalendarEventKind string

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
	Title                     string            `json:"title"`
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
	snap, err := s.Reports.GetHouseholdReport(ctx, householdID)
	if err != nil {
		return CalendarProjection{}, err
	}
	if from == 0 && to == 0 {
		from = snap.CurrentGameDay
		to = from + 182
	}
	if from < 0 || to < from || to-from > 364 {
		return CalendarProjection{}, fmt.Errorf("calendar range must be ordered and no longer than one year")
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
		events = append(events, sourceEvents(contextValue, from, to)...)
	}
	events = filterCalendarEvents(events, category)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].GameDay != events[j].GameDay {
			return events[i].GameDay < events[j].GameDay
		}
		if importanceRank(events[i].Importance) != importanceRank(events[j].Importance) {
			return importanceRank(events[i].Importance) < importanceRank(events[j].Importance)
		}
		return events[i].ID < events[j].ID
	})
	next := calendar.StartOfNextHalfYear(current)
	return CalendarProjection{
		HouseholdID: householdID, WorldID: snap.WorldID, StartYear: snap.SettingStartYear,
		CurrentGameDay: snap.CurrentGameDay, Current: calendar.Breakdown(current),
		NextHalfYear: NextHalfYear{Type: calendar.HalfYearAt(next), GameDay: int64(next), DaysUntil: calendar.DaysUntil(current, next)},
		FromGameDay:  from, ToGameDay: to, Events: events,
	}, nil
}

func seasonalEvents(from, to int64) []CalendarEvent {
	var events []CalendarEvent
	firstYear := calendar.YearIndex(calendar.GameDay(from)) - 1
	lastYear := calendar.YearIndex(calendar.GameDay(to)) + 1
	starts := []struct {
		season calendar.ProductionSeason
		day    int64
	}{
		// Summer and winter are represented by the named anchor rules below;
		// keeping spring and autumn here avoids duplicate events at those
		// culturally important boundaries.
		{calendar.Spring, 0}, {calendar.Autumn, 182},
	}
	for year := firstYear; year <= lastYear; year++ {
		for _, start := range starts {
			day := year*calendar.DaysPerYear + start.day
			if day < from || day > to {
				continue
			}
			events = append(events, CalendarEvent{ID: fmt.Sprintf("season-%d", day), Kind: CalendarSeasonStart, Category: "world", GameDay: day, Importance: "important", Title: fmt.Sprintf("%s begins", start.season)})
		}
	}
	return events
}

func anchorEvents(from, to int64) []CalendarEvent {
	var events []CalendarEvent
	firstYear := calendar.YearIndex(calendar.GameDay(from)) - 1
	lastYear := calendar.YearIndex(calendar.GameDay(to)) + 1
	for _, rule := range calendar.DefaultAnchors() {
		for year := firstYear; year <= lastYear; year++ {
			day := int64(calendar.AnchorGameDay(rule, year))
			if day < from || day > to {
				continue
			}
			kind := CalendarEventKind(rule.Kind)
			category, importance := "festivals", "context"
			title := anchorTitle(rule.Code)
			if rule.Kind == calendar.AnchorSeasonStart {
				category, importance, kind = "world", "important", CalendarSeasonStart
			} else if rule.Kind == calendar.AnchorHarvest {
				category, importance, kind = "farm", "important", CalendarHarvest
			} else if rule.Kind == calendar.AnchorAssembly {
				category, kind = "festivals", CalendarAssembly
			}
			events = append(events, CalendarEvent{ID: fmt.Sprintf("anchor-%s-%d", rule.Code, year), Kind: kind, Category: category, GameDay: day, Importance: importance, Title: title})
		}
	}
	return events
}

func sourceEvents(value port.CalendarContext, from, to int64) []CalendarEvent {
	var events []CalendarEvent
	for _, item := range value.Obligations {
		if item.DueGameDay >= from && item.DueGameDay <= to {
			events = append(events, CalendarEvent{ID: "obligation-" + item.ID, Kind: CalendarDeliveryDue, Category: "trade", GameDay: item.DueGameDay, Importance: "important", RelatedID: item.ID, ResourceType: item.ResourceType, QuantityMilli: item.QuantityMilli, CounterpartyHouseholdID: oppositeHousehold(value.Snapshot.HouseholdID, item.DebtorHouseholdID, item.CreditorHouseholdID), CounterpartyHouseholdName: item.CounterpartyName, Status: item.Status, Title: "Delivery due"})
		}
		if item.ShipmentID == "" && item.DebtorHouseholdID == value.Snapshot.HouseholdID {
			deadline := item.LatestDispatchGameDay
			if deadline >= from && deadline <= to {
				events = append(events, CalendarEvent{ID: "dispatch-" + item.ID, Kind: CalendarDispatchDeadline, Category: "trade", GameDay: deadline, Importance: "critical", ActionRequired: true, RelatedID: item.ID, ResourceType: item.ResourceType, QuantityMilli: item.QuantityMilli, CounterpartyHouseholdName: item.CounterpartyName, Status: item.Status, Title: "Dispatch shipment"})
			}
		}
	}
	for _, item := range value.Shipments {
		if item.ExpectedArrivalGameDay < from || item.ExpectedArrivalGameDay > to || item.ActualArrivalGameDay != nil {
			continue
		}
		events = append(events, CalendarEvent{ID: "shipment-" + item.ID, Kind: CalendarShipmentArrival, Category: "trade", GameDay: item.ExpectedArrivalGameDay, Importance: "important", RelatedID: item.ID, ResourceType: item.ResourceType, QuantityMilli: item.QuantityMilli, CounterpartyHouseholdName: item.CounterpartyName, Status: item.Status, Title: "Shipment expected"})
	}
	for _, item := range value.Deadlines {
		day := tickGameDay(value.Snapshot, item.DeadlineTick)
		if day >= from && day <= to {
			events = append(events, CalendarEvent{ID: item.ID, Kind: CalendarEventKind(item.Kind), Category: item.Category, GameDay: day, Importance: item.Importance, ActionRequired: true, RelatedID: item.ID, Title: item.Title})
		}
	}
	for _, item := range value.Assignments {
		day := tickGameDay(value.Snapshot, item.EndsTick)
		if day >= from && day <= to {
			events = append(events, CalendarEvent{ID: "assignment-" + item.ID, Kind: CalendarAssignmentEnd, Category: "farm", GameDay: day, Importance: item.Importance, RelatedID: item.ID, Title: "Work plan ends"})
		}
	}
	return events
}

func oppositeHousehold(current, debtor, creditor string) string {
	if current == debtor {
		return creditor
	}
	return debtor
}

func anchorTitle(code string) string {
	switch code {
	case "jol":
		return "Jól"
	case "thing":
		return "Þing"
	case "midsummer":
		return "Midsummer"
	case "harvest_start":
		return "Harvest begins"
	case "summer_start":
		return "Summer begins"
	case "winter_start":
		return "Winter begins"
	case "midwinter":
		return "Midwinter"
	default:
		return code
	}
}

func filterCalendarEvents(events []CalendarEvent, category string) []CalendarEvent {
	if category == "" || category == "all" {
		return events
	}
	if category == "festivals_and_assemblies" {
		category = "festivals"
	}
	filtered := events[:0]
	for _, event := range events {
		if event.Category == category {
			filtered = append(filtered, event)
		}
	}
	return filtered
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

func tickGameDay(snap port.HouseholdSnapshot, tick int64) int64 {
	delta := tick - snap.CurrentTick
	if snap.GameDaysPerTickNum <= 0 || snap.GameDaysPerTickDen <= 0 {
		return snap.CurrentGameDay
	}
	if delta >= 0 {
		return snap.CurrentGameDay + (snap.CalendarRemainder+delta*snap.GameDaysPerTickNum)/snap.GameDaysPerTickDen
	}
	return snap.CurrentGameDay - ((-delta)*snap.GameDaysPerTickNum-snap.CalendarRemainder+snap.GameDaysPerTickDen-1)/snap.GameDaysPerTickDen
}
