package port

import (
	"context"
	"time"

	contractdomain "game/backend/internal/domain/contract"
	marketdomain "game/backend/internal/domain/market"
	relationshipdomain "game/backend/internal/domain/relationship"
	shipmentdomain "game/backend/internal/domain/shipment"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/simulation"
)

type SimulationModel string

const (
	ModelLegacy     SimulationModel = "legacy"
	ModelDailyLabor SimulationModel = "daily_labor_v1"
)

type CharacterRecord struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BirthGameDay int64  `json:"birth_game_day"`
	// BirthDate is retained only as a decoding compatibility field for older
	// fixtures. PostgreSQL-backed projections no longer populate it.
	BirthDate      string                 `json:"-"`
	Age            int                    `json:"age"`
	LaborPermille  int64                  `json:"labor_permille"`
	Fatigue        int                    `json:"fatigue"`
	Status         string                 `json:"status"`
	Specialization string                 `json:"specialization,omitempty"`
	Occupation     *workdomain.Occupation `json:"occupation,omitempty"`
}

type AssignmentRecord struct {
	ID            string `json:"id"`
	CharacterID   string `json:"character_id"`
	Character     string `json:"character"`
	Activity      string `json:"activity"`
	Intensity     string `json:"intensity"`
	StartsTick    int64  `json:"starts_tick"`
	EndsTick      int64  `json:"ends_tick"`
	StartsGameDay int64  `json:"starts_game_day,omitempty"`
	EndsGameDay   int64  `json:"ends_game_day,omitempty"`
	Status        string `json:"status"`
}

type ShipmentRecord struct {
	ID                     string `json:"id"`
	WorldID                string `json:"world_id"`
	SenderHouseholdID      string `json:"sender_household_id"`
	SenderHouseholdName    string `json:"sender_household_name"`
	ReceiverHouseholdID    string `json:"receiver_household_id"`
	ReceiverHouseholdName  string `json:"receiver_household_name"`
	OriginLocationID       string `json:"origin_location_id"`
	DestinationLocationID  string `json:"destination_location_id"`
	ResourceType           string `json:"resource_type"`
	QuantityMilli          int64  `json:"quantity_milli"`
	DepartureTick          int64  `json:"departure_tick"`
	ExpectedArrivalTick    int64  `json:"expected_arrival_tick"`
	ActualArrivalTick      *int64 `json:"actual_arrival_tick,omitempty"`
	DepartureGameDay       int64  `json:"departure_game_day"`
	ExpectedArrivalGameDay int64  `json:"expected_arrival_game_day"`
	ActualArrivalGameDay   *int64 `json:"actual_arrival_game_day,omitempty"`
	TransportCostMilli     int64  `json:"transport_cost_milli"`
	Status                 string `json:"status"`
}

type MarketOfferRecord struct {
	ID                     string `json:"id"`
	WorldID                string `json:"world_id"`
	SellerHouseholdID      string `json:"seller_household_id"`
	SellerHouseholdName    string `json:"seller_household_name"`
	OriginLocationID       string `json:"origin_location_id"`
	ResourceType           string `json:"resource_type"`
	QuantityRemainingMilli int64  `json:"quantity_remaining_milli"`
	PricePerUnitMilli      int64  `json:"price_per_unit_milli"`
	CreatedTick            int64  `json:"created_tick"`
	ExpiresTick            *int64 `json:"expires_tick,omitempty"`
	Status                 string `json:"status"`
}

type ChronicleEntryRecord struct {
	Sequence                   int64          `json:"-"`
	ID                         string         `json:"id"`
	OccurredTick               int64          `json:"occurred_tick"`
	OccurredGameDay            int64          `json:"occurred_game_day"`
	EntryType                  string         `json:"entry_type"`
	SubjectCharacterID         *string        `json:"subject_character_id,omitempty"`
	SubjectCharacterName       *string        `json:"subject_character_name,omitempty"`
	RelatedHouseholdID         *string        `json:"related_household_id,omitempty"`
	RelatedHouseholdName       *string        `json:"related_household_name,omitempty"`
	RelatedShipmentID          *string        `json:"related_shipment_id,omitempty"`
	RelatedAssignmentID        *string        `json:"related_assignment_id,omitempty"`
	RelatedContractID          *string        `json:"related_contract_id,omitempty"`
	RelatedObligationID        *string        `json:"related_obligation_id,omitempty"`
	RelatedHouseholdDecisionID *string        `json:"related_household_decision_id,omitempty"`
	RelatedPoliticalActorID    *string        `json:"related_political_actor_id,omitempty"`
	Data                       map[string]any `json:"data"`
}

type HouseholdSnapshot struct {
	HouseholdID               string
	HouseholdName             string
	WorldID                   string
	WorldName                 string
	SimulationModel           SimulationModel
	CurrentTick               int64
	CurrentGameDay            int64
	CalendarRemainder         int64
	GameDaysPerTickNum        int64
	GameDaysPerTickDen        int64
	SettingStartYear          int32
	HistoricalStart           time.Time
	HistoricalDaysPerTickNum  int32
	HistoricalDaysPerTickDen  int32
	TickDurationSeconds       int32
	Specialization            string
	LastSeenGameDay           int64
	LastSeenChronicleSequence int64
	State                     simulation.HouseholdState
	Characters                []CharacterRecord
	Assignments               []AssignmentRecord
	DailyLabor                *simulation.DailyLaborState
}

// These deliberately small ports keep application services independent from
// PostgreSQL and allow deterministic service tests without a database.
type ReportReader interface {
	GetHouseholdReport(context.Context, string) (HouseholdSnapshot, error)
}

type HouseholdNameReader interface {
	HouseholdNames(context.Context, []string) (map[string]string, error)
}

type PoliticalReportDemand struct {
	ID             string
	ActorName      string
	ExpiresTick    int64
	ExpiresGameDay int64
}

type ContractReportObligation struct {
	ID                     string
	ResourceType           string
	QuantityMilli          int64
	DueArrivalTick         int64
	ExpectedArrivalTick    *int64
	DueGameDay             int64
	ExpectedArrivalGameDay *int64
}

// FarmReportReader is the narrow read model port used to enrich the household
// snapshot with recent facts and actionable obligations.
type FarmReportReader interface {
	ReportReader
	ListRecentChronicleForReport(context.Context, string, int64, int) ([]ChronicleEntryRecord, error)
	ListPendingPoliticalDemandsForReport(context.Context, string) ([]PoliticalReportDemand, error)
	ListContractObligationsForReport(context.Context, string) ([]ContractReportObligation, error)
	ListChronicleSinceGameDayForReport(context.Context, string, int64, int) ([]ChronicleEntryRecord, error)
}

// PreciseFarmReportReader uses the database's monotonic chronicle sequence so
// events created on the same game day, including concurrent events, are not
// lost between report loads.
type PreciseFarmReportReader interface {
	ListChronicleSinceCursor(context.Context, string, int64, int64, int) ([]ChronicleEntryRecord, error)
	CurrentChronicleCursor(context.Context, string) (int64, error)
}

type ReportAcknowledgementWriter interface {
	AcknowledgeHouseholdReport(context.Context, string, int64) error
}

type PreciseReportAcknowledgementWriter interface {
	AcknowledgeHouseholdReportCursor(context.Context, string, int64) error
}

type ShipmentRepository interface {
	CreateShipment(context.Context, shipmentdomain.Shipment) (shipmentdomain.Shipment, error)
	CancelShipment(context.Context, shipmentdomain.ID, shipmentdomain.HouseholdID) (shipmentdomain.Shipment, error)
	ListHouseholdShipments(context.Context, string) ([]ShipmentRecord, error)
}

type ChronicleReader interface {
	ListHouseholdChronicle(context.Context, string) ([]ChronicleEntryRecord, error)
}

type MarketPurchaseSnapshot struct {
	Offer              marketdomain.Offer
	Buyer              marketdomain.Buyer
	Route              marketdomain.Route
	SellerStockMilli   marketdomain.QuantityMilli
	CurrentTick        int64
	CurrentGameDay     int64
	CalendarRemainder  int64
	GameDaysPerTickNum int64
	GameDaysPerTickDen int64
}

// MarketPurchaseTransaction is scoped to the atomic lock/evaluate/persist
// workflow. It intentionally exposes no general SQL transaction operations.
type MarketPurchaseTransaction interface {
	Load(context.Context, string, string) (MarketPurchaseSnapshot, error)
	Persist(context.Context, marketdomain.Purchase, shipmentdomain.Shipment) (MarketOfferRecord, ShipmentRecord, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type MarketRepository interface {
	BeginMarketPurchase(context.Context) (MarketPurchaseTransaction, error)
	ListActiveMarketOffers(context.Context, string) ([]MarketOfferRecord, error)
}

type WorldClaim struct {
	ID                  string
	CurrentTick         int64
	CurrentGameDay      int64
	CalendarRemainder   int64
	GameDaysPerTickNum  int64
	GameDaysPerTickDen  int64
	TickDurationSeconds int32
	NextTickAt          time.Time
	SimulationModel     SimulationModel
}

type EmergencyFoodContext struct {
	Assignments       []AssignmentRecord
	IncomingShipments []ShipmentRecord
}

type ContractObligationAssessment struct {
	WorldID              contractdomain.WorldID
	Obligation           contractdomain.Obligation
	ActualArrivalTick    *shipmentdomain.Tick
	ActualArrivalGameDay *contractdomain.GameDay
	GameDaySchedule      bool
}

type ContractRollupSnapshot struct {
	Contract    contractdomain.Contract
	Obligations []contractdomain.Obligation
}

// WorldTickTransaction contains only the operations required by the canonical
// atomic tick. The ordering remains application-owned, not persistence-owned.
type WorldTickTransaction interface {
	PoliticsTickStore
	ClaimDueWorld(context.Context) (WorldClaim, bool, error)
	IsTickProcessed(context.Context, string, int64) (bool, error)
	LoadDueShipments(context.Context, string, int64) ([]shipmentdomain.Shipment, error)
	PersistShipmentArrival(context.Context, shipmentdomain.Shipment) (bool, error)
	LoadContractObligationsForTick(context.Context, string, int64, int64) ([]ContractObligationAssessment, error)
	PersistContractObligationAssessment(context.Context, contractdomain.Obligation, contractdomain.Obligation, *relationshipdomain.Event) (bool, error)
	LoadActiveContractsForRollup(context.Context, string) ([]ContractRollupSnapshot, error)
	PersistContractRollup(context.Context, contractdomain.Contract, contractdomain.Contract) (bool, error)
	ListHouseholdIDs(context.Context, string) ([]string, error)
	LoadHouseholdForTick(context.Context, string, int64) (HouseholdSnapshot, []simulation.Assignment, error)
	SaveHouseholdTick(context.Context, string, simulation.TickResult, int64) error
	SaveHouseholdDailyTick(context.Context, string, simulation.HourResult) error
	LoadEmergencyFoodContext(context.Context, string, int64) (EmergencyFoodContext, error)
	ScheduleEmergencyFoodWork(context.Context, string, EmergencyFoodDecisionRecord) (bool, error)
	RecordEmergencyFoodDecision(context.Context, string, EmergencyFoodDecisionRecord) error
	FinishWorldTick(context.Context, WorldClaim, int64, int64, int64) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type EmergencyFoodDecisionRecord struct {
	CharacterID             string
	Activity                string
	StartsTick              int64
	EndsTick                int64
	OccurredTick            int64
	OccurredGameDay         int64
	Reason                  string
	SupplyGameDays          int64
	ExpectedProductionMilli int64
	RemainsAtRisk           bool
}

type TickRepository interface {
	BeginWorldTick(context.Context) (WorldTickTransaction, error)
}
