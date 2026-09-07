package port

import (
	"context"

	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
)

type WorkPlan struct {
	HouseholdID     string
	CurrentGameDay  int64
	CurrentTick     int64
	CurrentMoment   calendar.Moment
	Occupations     []workdomain.Occupation
	TemporaryDuties []workdomain.TemporaryDuty
}

type OccupationChangeContext struct {
	HouseholdID     string
	WorldID         string
	Model           SimulationModel
	CurrentTick     int64
	Clock           calendar.ClockState
	CharacterID     string
	CharacterName   string
	CharacterStatus string
	LaborPermille   int64
	Occupation      workdomain.Occupation
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
