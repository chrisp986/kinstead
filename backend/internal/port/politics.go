package port

import (
	"context"
	"errors"
)

// ErrConcurrentTransaction indicates that PostgreSQL aborted a serializable
// transaction because another transaction changed the same authoritative state.
// Callers may retry the complete transaction from a fresh snapshot.
var ErrConcurrentTransaction = errors.New("concurrent transaction conflict")

type PoliticalEventRecord struct {
	ID, WorldID, LocationID, PoliticalActorID, EventType, ActorName, ActorType string
	StartsTick, ExpiresTick, StartsGameDay, ExpiresGameDay                     int64
	Parameters                                                                 []byte
}
type PoliticalDecisionRecord struct {
	ID, HouseholdID, WorldID, WorldEventID, DecisionType, Status string
	PoliticalActorID, EventType                                  string
	AvailableFromTick, ExpiresTick, CurrentTick                  int64
	AvailableFromGameDay, ExpiresGameDay, CurrentGameDay         int64
	SelectedOption                                               *string
	StandingDelta                                                *int
	Parameters                                                   []byte
}
type PoliticalCharacterRecord struct {
	ID, HouseholdID, Status string
	LaborCapacityMilli      int64
}
type PoliticalRelationshipRecord struct {
	PoliticalActorID string `json:"political_actor_id"`
	ActorName        string `json:"actor_name"`
	ActorType        string `json:"actor_type"`
	Score            int    `json:"score"`
	UpdatedAt        string `json:"updated_at,omitempty"`
	Standing         string `json:"standing"`
}
type PoliticalOption struct {
	Code              string `json:"code"`
	ResourceCode      string `json:"resource_code,omitempty"`
	ResourceMilli     int64  `json:"resource_milli,omitempty"`
	StandingDelta     int    `json:"standing_delta"`
	ServiceTicks      int64  `json:"service_ticks,omitempty"`
	RequiresCharacter bool   `json:"requires_character,omitempty"`
}
type PoliticalServiceCandidate struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	LaborPermille int64  `json:"labor_permille"`
}
type PoliticalDecisionProjection struct {
	ID                   string                      `json:"id"`
	DemandType           string                      `json:"demand_type"`
	Status               string                      `json:"status"`
	ActorID              string                      `json:"actor_id"`
	ActorName            string                      `json:"actor_name"`
	ActorType            string                      `json:"actor_type"`
	AvailableFromTick    int64                       `json:"available_from_tick"`
	ExpiresTick          int64                       `json:"expires_tick"`
	AvailableFromGameDay int64                       `json:"available_from_game_day"`
	ExpiresGameDay       int64                       `json:"expires_game_day"`
	SelectedOption       *string                     `json:"selected_option,omitempty"`
	StandingDelta        *int                        `json:"standing_delta,omitempty"`
	Parameters           map[string]any              `json:"parameters"`
	Options              []PoliticalOption           `json:"options"`
	EligibleCharacters   []PoliticalServiceCandidate `json:"eligible_characters,omitempty"`
}
type HouseholdPoliticsProjection struct {
	Relationships []PoliticalRelationshipRecord `json:"relationships"`
	Decisions     []PoliticalDecisionProjection `json:"decisions"`
}

type PoliticsTickStore interface {
	LoadPoliticalEventsStartingTick(context.Context, string, int64) ([]PoliticalEventRecord, error)
	ListHouseholdsForPoliticalEvent(context.Context, string) ([]string, error)
	InsertPoliticalDecision(context.Context, PoliticalDecisionRecord) (bool, error)
	LoadExpiringPoliticalDecisions(context.Context, string, int64) ([]PoliticalDecisionRecord, error)
	AutoResolvePoliticalDecision(context.Context, PoliticalDecisionRecord, int64, string, int) (bool, error)
	ApplyPoliticalScoreDelta(context.Context, string, string, string, int) error
	InsertPoliticalChronicle(context.Context, string, int64, string, string, string, string, []byte) error
	InsertPoliticalReceivedChronicle(context.Context, string, int64, string, string, []byte) error
}

type PoliticsReader interface {
	GetHouseholdPolitics(context.Context, string) (HouseholdPoliticsProjection, error)
}

type PoliticsResponseTransaction interface {
	LoadPoliticalDecision(context.Context, string, string) (PoliticalDecisionRecord, error)
	LoadCharacterForPolitics(context.Context, string, string) (PoliticalCharacterRecord, error)
	ResourceQuantity(context.Context, string, string) (int64, error)
	DeductResource(context.Context, string, string, int64) error
	AssignmentOverlaps(context.Context, string, int64, int64) (bool, error)
	CreateRulerServiceAssignment(context.Context, string, string, int64, int64, string) (string, error)
	ResolvePoliticalDecision(context.Context, string, string, int64, int, string) (bool, error)
	ApplyPoliticalScoreDelta(context.Context, string, string, string, int) error
	InsertPoliticalChronicle(context.Context, string, int64, string, string, string, string, []byte) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type PoliticsRepository interface {
	BeginPoliticsResponse(context.Context) (PoliticsResponseTransaction, error)
}
