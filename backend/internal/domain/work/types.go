package work

import (
	"errors"
	"fmt"

	"game/backend/internal/calendar"
)

type Activity string

const (
	Agriculture  Activity = "agriculture"
	Fishing      Activity = "fishing"
	Woodcutting  Activity = "woodcutting"
	Rest         Activity = "rest"
	RulerService Activity = "ruler_service"
)

var ErrInvalidOccupation = errors.New("invalid occupation")

func ValidateOccupation(activity Activity) error {
	switch activity {
	case Agriculture, Fishing, Woodcutting:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidOccupation, activity)
	}
}

type Occupation struct {
	CharacterID     string            `json:"character_id"`
	Activity        Activity          `json:"activity"`
	PendingActivity *Activity         `json:"pending_activity,omitempty"`
	EffectiveDay    *calendar.GameDay `json:"effective_game_day,omitempty"`
	EffectiveHour   *int              `json:"effective_hour,omitempty"`
	Revision        int64             `json:"revision"`
}

type TemporaryDuty struct {
	ID          string          `json:"id"`
	CharacterID string          `json:"character_id"`
	Activity    Activity        `json:"activity"`
	Starts      calendar.Moment `json:"starts"`
	Ends        calendar.Moment `json:"ends"` // end-exclusive
	Description string          `json:"description,omitempty"`
}

func (d TemporaryDuty) ActiveAt(moment calendar.Moment) bool {
	return (moment.Day > d.Starts.Day || (moment.Day == d.Starts.Day && moment.Hour >= d.Starts.Hour)) &&
		(moment.Day < d.Ends.Day || (moment.Day == d.Ends.Day && moment.Hour < d.Ends.Hour))
}

type WorkResolutionInput struct {
	Moment          calendar.Moment
	Workday         calendar.Workday
	Status          string
	LaborPermille   int64
	Occupation      Occupation
	TemporaryDuties []TemporaryDuty
	PolicyActivity  *Activity
	PolicyReason    string
}

type EffectiveWork struct {
	Eligible      bool
	Working       bool
	Recovering    bool
	BlockedByDuty bool
	Activity      Activity
	Reason        string
	DutyID        string
}

// ResolveEffectiveWork is the shared decision boundary used by execution
// and forecasts. Occupations describe the normal role; temporary duties and
// reserve protection can temporarily change what happens in an interval.
func ResolveEffectiveWork(input WorkResolutionInput) (EffectiveWork, error) {
	if err := ValidateOccupation(input.Occupation.Activity); err != nil {
		return EffectiveWork{}, err
	}
	if input.LaborPermille <= 0 || input.Status == "dead" {
		return EffectiveWork{Reason: "not eligible for home work"}, nil
	}
	if input.Status != "active" {
		return EffectiveWork{Eligible: true, Recovering: true, Reason: "unavailable character recovers"}, nil
	}
	for _, duty := range input.TemporaryDuties {
		if duty.ActiveAt(input.Moment) {
			return EffectiveWork{Eligible: true, BlockedByDuty: true, Recovering: false, Activity: duty.Activity, Reason: duty.Description, DutyID: duty.ID}, nil
		}
	}
	if !input.Workday.IsWorkingHour(input.Moment.Hour) {
		return EffectiveWork{Eligible: true, Recovering: true, Activity: Rest, Reason: "outside working hours"}, nil
	}
	activity := input.Occupation.Activity
	reason := "persistent occupation"
	if input.PolicyActivity != nil {
		activity = *input.PolicyActivity
		reason = input.PolicyReason
		if reason == "" {
			reason = "reserve policy"
		}
	}
	return EffectiveWork{Eligible: true, Working: activity != Rest, Activity: activity, Reason: reason}, nil
}
