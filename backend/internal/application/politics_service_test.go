package application

import (
	"context"
	"errors"
	"testing"

	politicsdomain "game/backend/internal/domain/politics"
	"game/backend/internal/port"
)

type fakePoliticsRepository struct {
	attempts     int
	commitErrors []error
	decisionFor  func(int) port.PoliticalDecisionRecord
	loadError    error
	character    port.PoliticalCharacterRecord
	characterErr error
	deductError  error
}

// A live response-vs-expiry race is scheduler-sensitive; these tests exercise
// the deterministic whole-transaction retry and resolved-state paths instead.

func (r *fakePoliticsRepository) BeginPoliticsResponse(context.Context) (port.PoliticsResponseTransaction, error) {
	r.attempts++
	return &fakePoliticsTransaction{
		repository: r, attempt: r.attempts,
		decision: r.decisionFor(r.attempts), loadError: r.loadError,
		character: r.character, characterErr: r.characterErr, deductError: r.deductError,
	}, nil
}

type fakePoliticsTransaction struct {
	repository   *fakePoliticsRepository
	attempt      int
	decision     port.PoliticalDecisionRecord
	loadError    error
	character    port.PoliticalCharacterRecord
	characterErr error
	deductError  error
}

func (t *fakePoliticsTransaction) LoadPoliticalDecision(context.Context, string, string) (port.PoliticalDecisionRecord, error) {
	return t.decision, t.loadError
}
func (t *fakePoliticsTransaction) LoadCharacterForPolitics(context.Context, string, string) (port.PoliticalCharacterRecord, error) {
	return t.character, t.characterErr
}
func (t *fakePoliticsTransaction) ResourceQuantity(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (t *fakePoliticsTransaction) DeductResource(context.Context, string, string, int64) error {
	return t.deductError
}
func (t *fakePoliticsTransaction) AssignmentOverlaps(context.Context, string, int64, int64) (bool, error) {
	return false, nil
}
func (t *fakePoliticsTransaction) CreateRulerServiceAssignment(context.Context, string, string, int64, int64, string) (string, error) {
	return "assignment", nil
}
func (t *fakePoliticsTransaction) ResolvePoliticalDecision(context.Context, string, string, int64, int, string) (bool, error) {
	return true, nil
}
func (t *fakePoliticsTransaction) ApplyPoliticalScoreDelta(context.Context, string, string, string, int) error {
	return nil
}
func (t *fakePoliticsTransaction) InsertPoliticalChronicle(context.Context, string, int64, string, string, string, string, []byte) error {
	return nil
}
func (t *fakePoliticsTransaction) Commit(context.Context) error {
	if t.attempt <= len(t.repository.commitErrors) {
		return t.repository.commitErrors[t.attempt-1]
	}
	return nil
}
func (t *fakePoliticsTransaction) Rollback(context.Context) error { return nil }

func politicalServiceTestDecision(status string) port.PoliticalDecisionRecord {
	return port.PoliticalDecisionRecord{
		ID: "decision", HouseholdID: "household", WorldID: "world", PoliticalActorID: "actor",
		EventType: string(politicsdomain.DemandLevy), DecisionType: string(politicsdomain.DemandLevy),
		Status: status, CurrentTick: 1, ExpiresTick: 5,
		Parameters: []byte("{\"wood_cost_milli\":18000,\"silver_cost_milli\":6000,\"honor_standing_delta\":10,\"refuse_standing_delta\":-5}"),
	}
}

func TestPoliticsResponseRetriesCompleteTransaction(t *testing.T) {
	repo := &fakePoliticsRepository{
		commitErrors: []error{port.ErrConcurrentTransaction, nil},
		decisionFor: func(int) port.PoliticalDecisionRecord {
			return politicalServiceTestDecision(string(politicsdomain.StatusPending))
		},
	}
	service := NewPoliticsService(repo, nil)
	if err := service.Respond(context.Background(), RespondPoliticalDemandCommand{DecisionID: "decision", HouseholdID: "household", Option: "refuse"}); err != nil {
		t.Fatal(err)
	}
	if repo.attempts != 2 {
		t.Fatalf("attempts=%d, want 2", repo.attempts)
	}
}

func TestPoliticsResponseStopsRetryWhenDemandWasResolved(t *testing.T) {
	repo := &fakePoliticsRepository{
		commitErrors: []error{port.ErrConcurrentTransaction},
		decisionFor: func(attempt int) port.PoliticalDecisionRecord {
			status := string(politicsdomain.StatusPending)
			if attempt == 2 {
				status = string(politicsdomain.StatusAutoResolved)
			}
			return politicalServiceTestDecision(status)
		},
	}
	service := NewPoliticsService(repo, nil)
	err := service.Respond(context.Background(), RespondPoliticalDemandCommand{DecisionID: "decision", HouseholdID: "household", Option: "refuse"})
	if !errors.Is(err, ErrPoliticalDemandResolved) {
		t.Fatalf("error=%v, want resolved conflict", err)
	}
	if repo.attempts != 2 {
		t.Fatalf("attempts=%d, want 2", repo.attempts)
	}
}

func TestPoliticsResponseDoesNotRetryExpectedErrors(t *testing.T) {
	tests := []struct {
		name      string
		cmd       RespondPoliticalDemandCommand
		configure func(*fakePoliticsRepository)
		want      error
	}{
		{name: "invalid option", cmd: RespondPoliticalDemandCommand{Option: "invalid"}, want: politicsdomain.ErrInvalidOption},
		{name: "expired", cmd: RespondPoliticalDemandCommand{Option: "refuse"}, configure: func(r *fakePoliticsRepository) {
			r.decisionFor = func(int) port.PoliticalDecisionRecord {
				d := politicalServiceTestDecision(string(politicsdomain.StatusPending))
				d.CurrentTick = d.ExpiresTick
				return d
			}
		}, want: politicsdomain.ErrExpired},
		{name: "insufficient resources", cmd: RespondPoliticalDemandCommand{Option: "pay_wood"}, configure: func(r *fakePoliticsRepository) {
			r.deductError = politicsdomain.ErrInsufficientResources
		}, want: politicsdomain.ErrInsufficientResources},
		{name: "ineligible worker", cmd: RespondPoliticalDemandCommand{Option: "serve", CharacterID: "character"}, configure: func(r *fakePoliticsRepository) {
			r.decisionFor = func(int) port.PoliticalDecisionRecord {
				d := politicalServiceTestDecision(string(politicsdomain.StatusPending))
				d.EventType = string(politicsdomain.DemandLaborService)
				d.DecisionType = string(politicsdomain.DemandLaborService)
				d.Parameters = []byte("{\"service_ticks\":4,\"honor_standing_delta\":10,\"refuse_standing_delta\":-5}")
				return d
			}
			r.character = port.PoliticalCharacterRecord{Status: "absent", LaborCapacityMilli: 1000}
		}, want: politicsdomain.ErrIneligibleCharacter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakePoliticsRepository{
				decisionFor: func(int) port.PoliticalDecisionRecord {
					return politicalServiceTestDecision(string(politicsdomain.StatusPending))
				},
			}
			if tt.configure != nil {
				tt.configure(repo)
			}
			service := NewPoliticsService(repo, nil)
			err := service.Respond(context.Background(), tt.cmd)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v, want %v", err, tt.want)
			}
			if repo.attempts != 1 {
				t.Fatalf("attempts=%d, want 1", repo.attempts)
			}
		})
	}
}

func TestPoliticsResponseUsesCalendarDeadlineWhenSnapshotIsPresent(t *testing.T) {
	repo := &fakePoliticsRepository{
		decisionFor: func(int) port.PoliticalDecisionRecord {
			d := politicalServiceTestDecision(string(politicsdomain.StatusPending))
			d.CurrentTick = 1
			d.ExpiresTick = 100
			d.CurrentGameDay = 10
			d.AvailableFromGameDay = 1
			d.ExpiresGameDay = 10
			return d
		},
	}
	service := NewPoliticsService(repo, nil)
	if err := service.Respond(context.Background(), RespondPoliticalDemandCommand{
		DecisionID: "decision", HouseholdID: "household", Option: "refuse",
	}); !errors.Is(err, politicsdomain.ErrExpired) {
		t.Fatalf("error=%v, want calendar deadline expiry", err)
	}
	if repo.attempts != 1 {
		t.Fatalf("attempts=%d, want 1", repo.attempts)
	}
}
