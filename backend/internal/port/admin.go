package port

import (
	"context"
	"time"

	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
)

const AdminPageSizeLimit = 100

type AdminCursor struct {
	Name string
	ID   string
}

type AdminHouseholdLink struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AdminWorld struct {
	ID                    string           `json:"id"`
	Name                  string           `json:"name"`
	CurrentTick           int64            `json:"current_tick"`
	CurrentGameDay        int64            `json:"current_game_day"`
	SimulationModel       SimulationModel  `json:"simulation_model"`
	TickDurationSeconds   int32            `json:"tick_duration_seconds"`
	NextTickAt            time.Time        `json:"next_tick_at"`
	LastCommittedAt       *time.Time       `json:"last_committed_at,omitempty"`
	DueTickCount          int64            `json:"due_tick_count"`
	Status                string           `json:"status"`
	StatusFacts           []string         `json:"status_facts"`
	Heartbeats            []AdminHeartbeat `json:"heartbeats"`
	LatestHeartbeat       *AdminHeartbeat  `json:"latest_heartbeat,omitempty"`
	RecentFailures        []AdminError     `json:"recent_failures"`
	CalendarAnchorAt      *time.Time       `json:"calendar_anchor_at,omitempty"`
	WorldUTCOffsetMinutes *int             `json:"world_utc_offset_minutes,omitempty"`
}

type AdminHeartbeat struct {
	InstanceID string    `json:"instance_id"`
	StartedAt  time.Time `json:"started_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type AdminCharacter struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	Status               string                 `json:"status"`
	LaborPermille        int64                  `json:"labor_permille"`
	Fatigue              int                    `json:"fatigue"`
	Age                  int                    `json:"age"`
	PersistentOccupation *workdomain.Occupation `json:"persistent_occupation,omitempty"`
	CurrentActivity      string                 `json:"current_activity"`
	ActivityReasonCode   string                 `json:"activity_reason_code"`
	ActivityReason       string                 `json:"activity_reason"`
	ExplanationMoment    *calendar.Moment       `json:"explanation_moment,omitempty"`
	TemporaryDutyIDs     []string               `json:"temporary_duty_ids"`
}

type AdminHousehold struct {
	ID                   string                     `json:"id"`
	Name                 string                     `json:"name"`
	OwnerPlayerID        *string                    `json:"owner_player_id,omitempty"`
	WorldID              string                     `json:"world_id"`
	WorldName            string                     `json:"world_name"`
	SnapshotCapturedAt   time.Time                  `json:"snapshot_captured_at"`
	CurrentCommittedTick int64                      `json:"current_committed_tick"`
	GameMoment           *calendar.Moment           `json:"game_moment,omitempty"`
	SimulationModel      SimulationModel            `json:"simulation_model"`
	Season               string                     `json:"season"`
	Workday              *calendar.Workday          `json:"workday,omitempty"`
	ResourcesMilli       map[string]int64           `json:"resources_milli"`
	PendingOutputMilli   map[string]int64           `json:"pending_output_milli"`
	NextSettlement       *calendar.Moment           `json:"next_settlement,omitempty"`
	Characters           []AdminCharacter           `json:"characters"`
	Occupations          []workdomain.Occupation    `json:"occupations"`
	TemporaryDuties      []workdomain.TemporaryDuty `json:"temporary_duties"`
	IncomingShipments    []ShipmentRecord           `json:"incoming_shipments"`
	OutgoingShipments    []ShipmentRecord           `json:"outgoing_shipments"`
	HistoryCoverage      AdminHistoryCoverage       `json:"history_coverage"`
}

type AdminHistoryCoverage struct {
	DiagnosticsRecordedFrom *time.Time `json:"diagnostics_recorded_from,omitempty"`
	RetainedDays            int        `json:"retained_days"`
	Statement               string     `json:"statement"`
}

type AdminTickDiagnostic struct {
	WorldID                 string          `json:"world_id"`
	HouseholdID             string          `json:"household_id"`
	Tick                    int64           `json:"tick"`
	IntervalStartDay        int64           `json:"interval_start_day"`
	IntervalStartHour       *int            `json:"interval_start_hour,omitempty"`
	IntervalEndDay          int64           `json:"interval_end_day"`
	IntervalEndHour         *int            `json:"interval_end_hour,omitempty"`
	SimulationModel         SimulationModel `json:"simulation_model"`
	DiagnosticSchemaVersion int             `json:"diagnostic_schema_version"`
	RecordedAt              time.Time       `json:"recorded_at"`
	Details                 map[string]any  `json:"details"`
}

type AdminError struct {
	ID               string    `json:"id"`
	OccurredAt       time.Time `json:"occurred_at"`
	Source           string    `json:"source"`
	ErrorCode        string    `json:"error_code"`
	Message          string    `json:"message"`
	RequestID        *string   `json:"request_id,omitempty"`
	WorkerInstanceID *string   `json:"worker_instance_id,omitempty"`
	WorldID          *string   `json:"world_id,omitempty"`
	HouseholdID      *string   `json:"household_id,omitempty"`
	Tick             *int64    `json:"tick,omitempty"`
	Stage            *string   `json:"stage,omitempty"`
	OccurrenceCount  int64     `json:"occurrence_count"`
	FirstObservedAt  time.Time `json:"first_observed_at"`
	LastObservedAt   time.Time `json:"last_observed_at"`
}

type AdminAccount struct {
	PlayerID            string               `json:"player_id"`
	ExternalAuthSubject string               `json:"external_auth_subject"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
	Households          []AdminHouseholdLink `json:"households"`
}

type AdminSession struct {
	ID         string     `json:"id"`
	PlayerID   string     `json:"player_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	Status     string     `json:"status"`
}

type AdminRepository interface {
	ListAdminWorlds(context.Context, AdminCursor, int) ([]AdminWorld, error)
	GetAdminWorld(context.Context, string) (AdminWorld, error)
	ListAdminHouseholds(context.Context, string, AdminCursor, int) ([]AdminHousehold, error)
	GetAdminHousehold(context.Context, string) (AdminHousehold, error)
	ListAdminTickDiagnostics(context.Context, string, int64, int) ([]AdminTickDiagnostic, error)
	GetAdminTickDiagnostic(context.Context, string, int64) (AdminTickDiagnostic, error)
	ListAdminErrors(context.Context, AdminErrorFilter, AdminCursor, int) ([]AdminError, error)
	ListAdminAccounts(context.Context, string, AdminCursor, int) ([]AdminAccount, error)
	GetAdminAccount(context.Context, string) (AdminAccount, error)
	ListAdminSessions(context.Context, string, AdminCursor, int) ([]AdminSession, error)
}

type AdminErrorFilter struct {
	WorldID     string
	HouseholdID string
	Tick        *int64
	Source      string
	RequestID   string
}
