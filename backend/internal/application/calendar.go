package application

import (
	"context"
	"fmt"

	"game/backend/internal/calendar"
	"game/backend/internal/port"
)

type CalendarEvent struct {
	ID       string `json:"id"`
	GameDay  int64  `json:"game_day"`
	Category string `json:"category"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
}

type CalendarProjection struct {
	HouseholdID string             `json:"household_id"`
	WorldID     string             `json:"world_id"`
	StartYear   int32              `json:"setting_start_year"`
	Current     calendar.Breakdown `json:"current"`
	FromGameDay int64              `json:"from_game_day"`
	ToGameDay   int64              `json:"to_game_day"`
	Events      []CalendarEvent    `json:"events"`
}

type CalendarService struct{ Store port.ReportReader }

func NewCalendarService(store port.ReportReader) *CalendarService {
	return &CalendarService{Store: store}
}

func (s *CalendarService) Household(ctx context.Context, householdID string, from, to int64, category string) (CalendarProjection, error) {
	snap, err := s.Store.GetHouseholdReport(ctx, householdID)
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
	events := make([]CalendarEvent, 0)
	for day := from; day <= to; day++ {
		value := calendar.GameDay(day)
		breakdown := calendar.BreakdownOf(value)
		if day == from || breakdown.ProductionSeason != calendar.ProductionSeasonAt(calendar.GameDay(day-1)) {
			season := string(breakdown.ProductionSeason)
			if category == "" || category == "season" {
				events = append(events, CalendarEvent{ID: fmt.Sprintf("season-%d", day), GameDay: day, Category: "season", Kind: "production_season", Title: season})
			}
		}
	}
	return CalendarProjection{
		HouseholdID: householdID, WorldID: snap.WorldID, StartYear: snap.SettingStartYear,
		Current:     calendar.BreakdownOf(calendar.GameDay(snap.CurrentGameDay)),
		FromGameDay: from, ToGameDay: to, Events: events,
	}, nil
}
