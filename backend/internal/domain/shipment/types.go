package shipment

import "errors"

type ID string
type WorldID string
type HouseholdID string
type LocationID string
type ResourceType string
type QuantityMilli int64
type MoneyMilli int64
type Tick int64
type Status string

const (
	StatusPrepared  Status = "prepared"
	StatusInTransit Status = "in_transit"
	StatusArrived   Status = "arrived"
	StatusCancelled Status = "cancelled"
)

var (
	ErrInvalidShipment   = errors.New("invalid shipment")
	ErrInvalidTransition = errors.New("invalid shipment status transition")
	ErrNotDue            = errors.New("shipment is not due")
)

type Shipment struct {
	ID                    ID
	WorldID               WorldID
	SenderHouseholdID     HouseholdID
	ReceiverHouseholdID   HouseholdID
	OriginLocationID      LocationID
	DestinationLocationID LocationID
	ResourceType          ResourceType
	QuantityMilli         QuantityMilli
	DepartureTick         Tick
	ExpectedArrivalTick   Tick
	ActualArrivalTick     *Tick
	TransportCostMilli    MoneyMilli
	Status                Status
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
		valid = to == StatusArrived || to == StatusCancelled
	}
	if !valid {
		return Shipment{}, ErrInvalidTransition
	}
	s.Status = to
	return s, nil
}

func (s Shipment) Arrive(tick Tick) (Shipment, error) {
	if !s.DueAt(tick) {
		return Shipment{}, ErrNotDue
	}
	arrived, err := s.Transition(StatusArrived)
	if err != nil {
		return Shipment{}, err
	}
	arrived.ActualArrivalTick = &tick
	return arrived, nil
}
