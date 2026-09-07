package application

import (
	"context"
	"errors"
	"fmt"

	"game/backend/internal/calendar"
	chronicle "game/backend/internal/domain/chronicle"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/port"
)

var (
	ErrUnsupportedSimulationModel = errors.New("unsupported simulation model")
	ErrOccupationRevisionConflict = errors.New("occupation revision conflict")
	ErrOccupationIneligible       = errors.New("character is not eligible for an occupation")
)

type ChangeOccupationCommand struct {
	HouseholdID      string
	CharacterID      string
	Activity         workdomain.Activity
	ExpectedRevision int64
}

type OccupationChangeResult struct {
	Occupation workdomain.Occupation `json:"occupation"`
	Effective  calendar.Moment       `json:"effective"`
	Changed    bool                  `json:"changed"`
}

type OccupationService struct {
	Repo port.OccupationRepository
}

func NewOccupationService(repo port.OccupationRepository) *OccupationService {
	return &OccupationService{Repo: repo}
}

func (s *OccupationService) WorkPlan(ctx context.Context, householdID string) (port.WorkPlan, error) {
	return s.Repo.GetWorkPlan(ctx, householdID)
}

func (s *OccupationService) Change(ctx context.Context, command ChangeOccupationCommand) (OccupationChangeResult, error) {
	if err := workdomain.ValidateOccupation(command.Activity); err != nil {
		return OccupationChangeResult{}, err
	}
	tx, err := s.Repo.BeginOccupationChange(ctx)
	if err != nil {
		return OccupationChangeResult{}, err
	}
	defer tx.Rollback(ctx)
	loaded, err := tx.LoadOccupationChangeContext(ctx, command.HouseholdID, command.CharacterID)
	if err != nil {
		return OccupationChangeResult{}, err
	}
	if loaded.Model != port.ModelDailyLabor {
		return OccupationChangeResult{}, ErrUnsupportedSimulationModel
	}
	if loaded.CharacterStatus == "dead" || loaded.LaborPermille <= 0 {
		return OccupationChangeResult{}, ErrOccupationIneligible
	}
	if command.ExpectedRevision != loaded.Occupation.Revision {
		return OccupationChangeResult{}, fmt.Errorf("%w: expected %d, current %d", ErrOccupationRevisionConflict, command.ExpectedRevision, loaded.Occupation.Revision)
	}
	effective, err := calendar.NextWorkStart(loaded.Clock)
	if err != nil {
		return OccupationChangeResult{}, err
	}
	updated := loaded.Occupation
	changed := false
	if command.Activity == updated.Activity {
		if updated.PendingActivity != nil {
			updated.PendingActivity = nil
			updated.EffectiveDay = nil
			changed = true // selecting the current role cancels a pending change
		}
	} else if updated.PendingActivity == nil || *updated.PendingActivity != command.Activity {
		activity := command.Activity
		updated.PendingActivity = &activity
		day := effective.Day
		updated.EffectiveDay = &day
		changed = true
	}
	if !changed {
		return OccupationChangeResult{Occupation: updated, Effective: effective, Changed: false}, nil
	}
	updated.Revision++
	if err := tx.SaveOccupation(ctx, updated); err != nil {
		return OccupationChangeResult{}, err
	}
	if err := tx.InsertOccupationChronicle(ctx, chronicle.OccupationChanged, loaded.CurrentTick, int64(loaded.Clock.Day), command.CharacterID, updated); err != nil {
		return OccupationChangeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OccupationChangeResult{}, err
	}
	return OccupationChangeResult{Occupation: updated, Effective: effective, Changed: true}, nil
}
