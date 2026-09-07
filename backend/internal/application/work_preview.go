package application

import (
	"context"
	"errors"
	"fmt"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

var ErrInvalidWorkPreview = errors.New("invalid work preview")

type WorkPreviewCommand struct {
	HouseholdID   string
	CharacterID   string
	Activity      simulation.Activity
	Intensity     simulation.Intensity
	DurationTicks int64
}

type WorkPreview struct {
	ExpectedProvisionsMilli        int64    `json:"produced_provisions_milli"`
	ExpectedWoodMilli              int64    `json:"produced_wood_milli"`
	StartingFatigue                int      `json:"fatigue_start"`
	EndingFatigue                  int      `json:"fatigue_end"`
	Season                         string   `json:"season"`
	SpecializationModifierPermille int64    `json:"specialization_bonus_permille"`
	FarmModifierPermille           int64    `json:"farm_bonus_permille"`
	Conflict                       bool     `json:"conflict"`
	Warning                        *string  `json:"warning"`
	DurationGameDays               int64    `json:"duration_game_days"`
	AssignmentConflicts            []string `json:"assignment_conflicts"`
	Warnings                       []string `json:"warnings"`
}

type WorkPreviewService struct {
	Store   port.ReportReader
	Balance simulation.BalanceConfig
}

func NewWorkPreviewService(store port.ReportReader) *WorkPreviewService {
	return &WorkPreviewService{Store: store, Balance: balance.V03()}
}

func (s *WorkPreviewService) Preview(ctx context.Context, cmd WorkPreviewCommand) (WorkPreview, error) {
	if cmd.HouseholdID == "" || cmd.CharacterID == "" || cmd.DurationTicks <= 0 || cmd.DurationTicks > 12 {
		return WorkPreview{}, ErrInvalidWorkPreview
	}
	if _, ok := s.Balance.Intensity[cmd.Intensity]; !ok {
		return WorkPreview{}, ErrInvalidWorkPreview
	}
	snap, err := s.Store.GetHouseholdReport(ctx, cmd.HouseholdID)
	if err != nil {
		return WorkPreview{}, err
	}
	idx, err := snap.State.CharacterIndexByID(cmd.CharacterID)
	if err != nil {
		return WorkPreview{}, ErrInvalidWorkPreview
	}
	character := snap.State.Characters[idx]
	if character.LaborPermille <= 0 {
		return WorkPreview{}, ErrInvalidWorkPreview
	}
	if cmd.Activity != simulation.Agriculture && cmd.Activity != simulation.Fishing && cmd.Activity != simulation.Woodcutting && cmd.Activity != simulation.Rest {
		return WorkPreview{}, ErrInvalidWorkPreview
	}
	preview := WorkPreview{StartingFatigue: character.Fatigue, AssignmentConflicts: []string{}, Warnings: []string{}}
	if character.Specialization == cmd.Activity {
		preview.SpecializationModifierPermille = s.Balance.SkillModifierPermille
	} else {
		preview.SpecializationModifierPermille = 1000
	}
	preview.FarmModifierPermille = 1000
	if mods, ok := s.Balance.FarmModifiers[snap.State.FarmSpecialization]; ok && mods[cmd.Activity] != 0 {
		preview.FarmModifierPermille = mods[cmd.Activity]
	}
	startTick, endTick := snap.CurrentTick+1, snap.CurrentTick+cmd.DurationTicks
	for _, existing := range snap.Assignments {
		if existing.CharacterID == cmd.CharacterID && existing.StartsTick <= endTick && existing.EndsTick >= startTick {
			preview.AssignmentConflicts = append(preview.AssignmentConflicts, existing.ID)
		}
	}
	startDay := snap.CurrentGameDay
	endDay, err := gameDayAfterTicks(snap.CurrentGameDay, snap.CalendarRemainder, snap.GameDaysPerTickNum, snap.GameDaysPerTickDen, cmd.DurationTicks)
	if err != nil {
		return WorkPreview{}, fmt.Errorf("preview duration: %w", err)
	}
	preview.DurationGameDays = endDay - startDay
	for offset := int64(0); offset < cmd.DurationTicks; offset++ {
		day, err := calendar.GameDayAfterTicks(calendar.GameDay(snap.CurrentGameDay), snap.CalendarRemainder, snap.GameDaysPerTickNum, snap.GameDaysPerTickDen, offset)
		if err != nil {
			return WorkPreview{}, err
		}
		season := simulation.Season(calendar.ProductionSeasonAt(day))
		if offset == 0 {
			preview.Season = string(season)
		}
		tickContext := simulation.NeutralTickContext(season)
		tickContext.GameDaysPerTickNum, tickContext.GameDaysPerTickDen = snap.GameDaysPerTickNum, snap.GameDaysPerTickDen
		assignment := simulation.Assignment{CharacterID: character.ID, Activity: cmd.Activity, Intensity: cmd.Intensity}
		produced := simulation.EstimateProduction(character, assignment, snap.State.FarmSpecialization, tickContext, s.Balance)
		switch cmd.Activity {
		case simulation.Agriculture, simulation.Fishing:
			preview.ExpectedProvisionsMilli += produced
		case simulation.Woodcutting:
			preview.ExpectedWoodMilli += produced
		}
		character.Fatigue = simulation.FatigueAfter(character.Fatigue, cmd.Activity, cmd.Intensity, s.Balance)
	}
	preview.EndingFatigue = character.Fatigue
	if simulation.FatigueProductionPermille(preview.EndingFatigue) < 1000 {
		preview.Warnings = append(preview.Warnings, "fatigue_reduces_output")
	} else if preview.EndingFatigue >= 50 {
		preview.Warnings = append(preview.Warnings, "fatigue_warning")
	}
	preview.Conflict = len(preview.AssignmentConflicts) > 0
	if preview.Conflict {
		warning := "assignment_conflict"
		preview.Warning = &warning
	} else if len(preview.Warnings) > 0 {
		preview.Warning = &preview.Warnings[0]
	}
	return preview, nil
}
