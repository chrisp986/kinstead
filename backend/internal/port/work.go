package port

import (
	"context"
	"time"

	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
)

type WorkPlan struct {
	HouseholdID           string                     `json:"household_id"`
	CurrentGameDay        int64                      `json:"current_game_day"`
	CurrentTick           int64                      `json:"current_tick"`
	CurrentMoment         calendar.Moment            `json:"current_moment"`
	Season                calendar.ProductionSeason  `json:"season"`
	SeasonDay             int                        `json:"season_day,omitempty"`
	SeasonLengthDays      int                        `json:"season_length_days,omitempty"`
	WorldUTCOffsetMinutes int                        `json:"world_utc_offset_minutes"`
	Workday               calendar.Workday           `json:"workday"`
	NextWorkingPeriod     calendar.Moment            `json:"next_working_period"`
	NextSettlement        calendar.Moment            `json:"next_settlement"`
	Occupations           []workdomain.Occupation    `json:"occupations"`
	TemporaryDuties       []workdomain.TemporaryDuty `json:"temporary_duties"`
}

type OccupationChangeContext struct {
	HouseholdID           string
	WorldID               string
	Model                 SimulationModel
	CurrentTick           int64
	Clock                 calendar.ClockState
	CalendarAnchorAt      time.Time
	WorldUTCOffsetMinutes int
	CharacterID           string
	CharacterName         string
	CharacterStatus       string
	LaborPermille         int64
	Occupation            workdomain.Occupation
}

type OccupationChangeTransaction interface {
	LoadOccupationChangeContext(context.Context, string, string) (OccupationChangeContext, error)
	SaveOccupation(context.Context, workdomain.Occupation) error
	InsertOccupationChronicle(context.Context, string, int64, int64, string, workdomain.Occupation) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type OccupationRepository interface {
	BeginOccupationChange(context.Context) (OccupationChangeTransaction, error)
	GetWorkPlan(context.Context, string) (WorkPlan, error)
}
