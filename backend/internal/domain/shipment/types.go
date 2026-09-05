package shipment

import (
	"errors"

	"game/backend/internal/calendar"
)

type ID string
type WorldID string
type HouseholdID string
type LocationID string
type ResourceType string
type QuantityMilli int64
type MoneyMilli int64
type Tick int64
type GameDay = calendar.GameDay
type Status string

const (
	StatusPrepared  Status = "prepared"
	StatusInTransit Status = "in_transit"
	StatusArrived   Status = "arrived"
	StatusCancelled Status = "cancelled"
)

var (
	ErrInvalidShipment       = errors.New("invalid shipment")
	ErrInvalidTransition     = errors.New("invalid shipment status transition")
	ErrNotDue                = errors.New("shipment is not due")
	ErrCancellationClosed    = errors.New("shipment can no longer be cancelled")
	ErrCancellationForbidden = errors.New("shipment cancellation is forbidden")
)

type Shipment struct {
	ID                     ID
	WorldID                WorldID
	SenderHouseholdID      HouseholdID
	ReceiverHouseholdID    HouseholdID
	OriginLocationID       LocationID
	DestinationLocationID  LocationID
	ResourceType           ResourceType
	QuantityMilli          QuantityMilli
	DepartureTick          Tick
	ExpectedArrivalTick    Tick
	ActualArrivalTick      *Tick
	DepartureGameDay       GameDay
	ExpectedArrivalGameDay GameDay
	ActualArrivalGameDay   *GameDay
	TransportCostMilli     MoneyMilli
	Status                 Status
}

func (s Shipment) Validate() error {
	if s.WorldID == "" || s.SenderHouseholdID == "" || s.ReceiverHouseholdID == "" ||
		s.OriginLocationID == "" || s.DestinationLocationID == "" || s.ResourceType == "" ||
		s.SenderHouseholdID == s.ReceiverHouseholdID || s.OriginLocationID == s.DestinationLocationID ||
		s.QuantityMilli <= 0 || s.TransportCostMilli < 0 || s.DepartureTick < 0 ||
		s.ExpectedArrivalTick <= s.DepartureTick {
		return ErrInvalidShipment
	}
	switch s.Status {
	case StatusPrepared, StatusInTransit, StatusCancelled:
		if s.ActualArrivalTick != nil {
			return ErrInvalidShipment
		}
	case StatusArrived:
		if s.ActualArrivalTick == nil || *s.ActualArrivalTick < s.ExpectedArrivalTick {
			return ErrInvalidShipment
		}
		if s.ActualArrivalGameDay != nil && *s.ActualArrivalGameDay < s.ExpectedArrivalGameDay {
			return ErrInvalidShipment
		}
	default:
		return ErrInvalidShipment
	}
	return nil
}

func (s Shipment) DueAt(tick Tick) bool {
	return s.Status == StatusInTransit && s.ActualArrivalTick == nil && s.ExpectedArrivalTick <= tick
}

func (s Shipment) Transition(to Status) (Shipment, error) {
	valid := false
	switch s.Status {
	case StatusPrepared:
		valid = to == StatusInTransit || to == StatusCancelled
	case StatusInTransit:
		valid = to == StatusArrived
	}
	if !valid {
		return Shipment{}, ErrInvalidTransition
	}
	s.Status = to
	return s, nil
}

// CancelAt cancels a direct transfer before it becomes due. Persistence is
// responsible for authorizing the sender and refunding reserved goods.
func (s Shipment) CancelAt(tick Tick) (Shipment, error) {
	if tick < 0 {
		return Shipment{}, ErrCancellationClosed
	}
	if s.Status == StatusPrepared {
		return s.Transition(StatusCancelled)
	}
	if s.Status != StatusInTransit || tick >= s.ExpectedArrivalTick {
		return Shipment{}, ErrCancellationClosed
	}
	s.Status = StatusCancelled
	return s, nil
}

func (s Shipment) Arrive(tick Tick) (Shipment, error) {
	return s.ArriveAt(tick, s.ExpectedArrivalGameDay)
}

// ArriveAt records both execution-time and calendar-time arrival snapshots.
// Arrive remains available to callers that only have the execution tick.
func (s Shipment) ArriveAt(tick Tick, gameDay GameDay) (Shipment, error) {
	if !s.DueAt(tick) {
		return Shipment{}, ErrNotDue
	}
	arrived, err := s.Transition(StatusArrived)
	if err != nil {
		return Shipment{}, err
	}
	arrived.ActualArrivalTick = &tick
	arrived.ActualArrivalGameDay = &gameDay
	return arrived, nil
}
