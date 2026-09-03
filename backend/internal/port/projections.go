package port

import (
	"context"
	"time"

	marketdomain "game/backend/internal/domain/market"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/simulation"
)

type CharacterRecord struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	BirthDate      string `json:"birth_date"`
	Age            int    `json:"age"`
	LaborPermille  int64  `json:"labor_permille"`
	Fatigue        int    `json:"fatigue"`
	Specialization string `json:"specialization,omitempty"`
}

type AssignmentRecord struct {
	ID          string `json:"id"`
	CharacterID string `json:"character_id"`
	Character   string `json:"character"`
	Activity    string `json:"activity"`
	Intensity   string `json:"intensity"`
	StartsTick  int64  `json:"starts_tick"`
	EndsTick    int64  `json:"ends_tick"`
	Status      string `json:"status"`
}

type ShipmentRecord struct {
	ID                    string `json:"id"`
	WorldID               string `json:"world_id"`
	SenderHouseholdID     string `json:"sender_household_id"`
	ReceiverHouseholdID   string `json:"receiver_household_id"`
	OriginLocationID      string `json:"origin_location_id"`
	DestinationLocationID string `json:"destination_location_id"`
	ResourceType          string `json:"resource_type"`
	QuantityMilli         int64  `json:"quantity_milli"`
	DepartureTick         int64  `json:"departure_tick"`
	ExpectedArrivalTick   int64  `json:"expected_arrival_tick"`
	ActualArrivalTick     *int64 `json:"actual_arrival_tick,omitempty"`
	TransportCostMilli    int64  `json:"transport_cost_milli"`
	Status                string `json:"status"`
}

type MarketOfferRecord struct {
	ID                     string `json:"id"`
	WorldID                string `json:"world_id"`
	SellerHouseholdID      string `json:"seller_household_id"`
	OriginLocationID       string `json:"origin_location_id"`
	ResourceType           string `json:"resource_type"`
	QuantityRemainingMilli int64  `json:"quantity_remaining_milli"`
	PricePerUnitMilli      int64  `json:"price_per_unit_milli"`
	CreatedTick            int64  `json:"created_tick"`
	ExpiresTick            *int64 `json:"expires_tick,omitempty"`
	Status                 string `json:"status"`
}

type ChronicleEntryRecord struct {
	ID                   string         `json:"id"`
	OccurredTick         int64          `json:"occurred_tick"`
	EntryType            string         `json:"entry_type"`
	SubjectCharacterID   *string        `json:"subject_character_id,omitempty"`
	SubjectCharacterName *string        `json:"subject_character_name,omitempty"`
	RelatedHouseholdID   *string        `json:"related_household_id,omitempty"`
	RelatedHouseholdName *string        `json:"related_household_name,omitempty"`
	RelatedShipmentID    *string        `json:"related_shipment_id,omitempty"`
	RelatedAssignmentID  *string        `json:"related_assignment_id,omitempty"`
	Data                 map[string]any `json:"data"`
}

type HouseholdSnapshot struct {
	HouseholdID              string
	HouseholdName            string
	WorldID                  string
	WorldName                string
	CurrentTick              int64
	HistoricalStart          time.Time
	HistoricalDaysPerTickNum int32
	HistoricalDaysPerTickDen int32
	Specialization           string
	State                    simulation.HouseholdState
	Characters               []CharacterRecord
	Assignments              []AssignmentRecord
}

// These deliberately small ports keep application services independent from
// PostgreSQL and allow deterministic service tests without a database.
type ReportReader interface {
	GetHouseholdReport(context.Context, string) (HouseholdSnapshot, error)
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
	Offer            marketdomain.Offer
	Buyer            marketdomain.Buyer
	Route            marketdomain.Route
	SellerStockMilli marketdomain.QuantityMilli
	CurrentTick      int64
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
	TickDurationSeconds int32
	NextTickAt          time.Time
}

// WorldTickTransaction contains only the operations required by the canonical
// atomic tick. The ordering remains application-owned, not persistence-owned.
type WorldTickTransaction interface {
	ClaimDueWorld(context.Context) (WorldClaim, bool, error)
	IsTickProcessed(context.Context, string, int64) (bool, error)
	LoadDueShipments(context.Context, string, int64) ([]shipmentdomain.Shipment, error)
	PersistShipmentArrival(context.Context, shipmentdomain.Shipment) (bool, error)
	ListHouseholdIDs(context.Context, string) ([]string, error)
	LoadHouseholdForTick(context.Context, string, int64) (HouseholdSnapshot, []simulation.Assignment, error)
	SaveHouseholdTick(context.Context, string, simulation.TickResult) error
	FinishWorldTick(context.Context, WorldClaim, int64) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type TickRepository interface {
	BeginWorldTick(context.Context) (WorldTickTransaction, error)
}
