package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/port"
)

type occupationRepoStub struct{ tx *occupationTxStub }

func (r occupationRepoStub) BeginOccupationChange(context.Context) (port.OccupationChangeTransaction, error) {
	return r.tx, nil
}
func (r occupationRepoStub) GetWorkPlan(context.Context, string) (port.WorkPlan, error) {
	return port.WorkPlan{}, nil
}

type occupationTxStub struct {
	loaded    port.OccupationChangeContext
	saved     *workdomain.Occupation
	committed bool
}

func (t *occupationTxStub) LoadOccupationChangeContext(context.Context, string, string) (port.OccupationChangeContext, error) {
	return t.loaded, nil
}
func (t *occupationTxStub) SaveOccupation(_ context.Context, value workdomain.Occupation) error {
	t.saved = &value
	return nil
}
func (t *occupationTxStub) InsertOccupationChronicle(context.Context, string, int64, int64, string, workdomain.Occupation) error {
	return nil
}
func (t *occupationTxStub) Commit(context.Context) error   { t.committed = true; return nil }
func (t *occupationTxStub) Rollback(context.Context) error { return nil }

func monthlyOccupationContext(hour int) port.OccupationChangeContext {
	anchor := time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC)
	return port.OccupationChangeContext{Model: port.ModelMonthlySeasons, Clock: calendar.ClockState{Day: 0, Remainder: int64(hour), GameDaysPerTickNum: 1, GameDaysPerTickDen: 24}, CalendarAnchorAt: anchor, CharacterStatus: "active", LaborPermille: 1000, Occupation: workdomain.Occupation{CharacterID: "worker", Activity: workdomain.Fishing, Revision: 2}}
}

func TestMonthlyOccupationChangeBeforeAndAtWorkStart(t *testing.T) {
	for _, tt := range []struct {
		name    string
		hour    int
		wantDay calendar.GameDay
	}{{"before", 8, 0}, {"at committed boundary", 9, 1}} {
		t.Run(tt.name, func(t *testing.T) {
			tx := &occupationTxStub{loaded: monthlyOccupationContext(tt.hour)}
			result, err := NewOccupationService(occupationRepoStub{tx}).Change(context.Background(), ChangeOccupationCommand{CharacterID: "worker", Activity: workdomain.Agriculture, ExpectedRevision: 2})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Changed || result.Effective.Day != tt.wantDay || result.Effective.Hour != 9 || tx.saved == nil || !tx.committed {
				t.Fatalf("result=%+v saved=%+v", result, tx.saved)
			}
		})
	}
}

func TestOccupationChangeConflictCancelAndNoOp(t *testing.T) {
	tx := &occupationTxStub{loaded: monthlyOccupationContext(8)}
	_, err := NewOccupationService(occupationRepoStub{tx}).Change(context.Background(), ChangeOccupationCommand{CharacterID: "worker", Activity: workdomain.Agriculture, ExpectedRevision: 1})
	if !errors.Is(err, ErrOccupationRevisionConflict) {
		t.Fatalf("conflict=%v", err)
	}

	pending := workdomain.Agriculture
	day, hour := calendar.GameDay(1), 9
	cancelTx := &occupationTxStub{loaded: monthlyOccupationContext(8)}
	cancelTx.loaded.Occupation.PendingActivity, cancelTx.loaded.Occupation.EffectiveDay, cancelTx.loaded.Occupation.EffectiveHour = &pending, &day, &hour
	result, err := NewOccupationService(occupationRepoStub{cancelTx}).Change(context.Background(), ChangeOccupationCommand{CharacterID: "worker", Activity: workdomain.Fishing, ExpectedRevision: 2})
	if err != nil || !result.Changed || result.Occupation.PendingActivity != nil || result.Occupation.EffectiveHour != nil {
		t.Fatalf("cancel=%+v err=%v", result, err)
	}

	noOpTx := &occupationTxStub{loaded: monthlyOccupationContext(8)}
	result, err = NewOccupationService(occupationRepoStub{noOpTx}).Change(context.Background(), ChangeOccupationCommand{CharacterID: "worker", Activity: workdomain.Fishing, ExpectedRevision: 2})
	if err != nil || result.Changed || noOpTx.saved != nil {
		t.Fatalf("no-op=%+v saved=%+v err=%v", result, noOpTx.saved, err)
	}
}
