package application

import (
	"context"
	"fmt"

	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
)

type CreateShipmentCommand struct {
	ID                     shipmentdomain.ID
	WorldID                shipmentdomain.WorldID
	SenderHouseholdID      shipmentdomain.HouseholdID
	ReceiverHouseholdID    shipmentdomain.HouseholdID
	OriginLocationID       shipmentdomain.LocationID
	DestinationLocationID  shipmentdomain.LocationID
	ResourceType           shipmentdomain.ResourceType
	QuantityMilli          shipmentdomain.QuantityMilli
	DepartureTick          shipmentdomain.Tick
	ExpectedArrivalTick    shipmentdomain.Tick
	DepartureGameDay       shipmentdomain.GameDay
	ExpectedArrivalGameDay shipmentdomain.GameDay
	TransportCostMilli     shipmentdomain.MoneyMilli
}

type CancelShipmentCommand struct {
	ShipmentID        shipmentdomain.ID
	SenderHouseholdID shipmentdomain.HouseholdID
}

type ShipmentService struct {
	Store port.ShipmentRepository
}

func (s *ShipmentService) Cancel(ctx context.Context, cmd CancelShipmentCommand) (port.ShipmentRecord, error) {
	if cmd.ShipmentID == "" || cmd.SenderHouseholdID == "" {
		return port.ShipmentRecord{}, shipmentdomain.ErrCancellationForbidden
	}
	value, err := s.Store.CancelShipment(ctx, cmd.ShipmentID, cmd.SenderHouseholdID)
	if err != nil {
		return port.ShipmentRecord{}, err
	}
	records, err := s.Store.ListHouseholdShipments(ctx, string(cmd.SenderHouseholdID))
	if err != nil {
		return port.ShipmentRecord{}, err
	}
	for _, record := range records {
		if record.ID == string(value.ID) {
			return record, nil
		}
	}
	return port.ShipmentRecord{}, fmt.Errorf("cancelled shipment %s missing from projection", value.ID)
}

func NewShipmentService(store port.ShipmentRepository) *ShipmentService {
	return &ShipmentService{Store: store}
}

// Create reserves the goods at the sender and starts an in-transit shipment.
func (s *ShipmentService) Create(ctx context.Context, cmd CreateShipmentCommand) (shipmentdomain.Shipment, error) {
	prepared := shipmentdomain.Shipment{
		ID: cmd.ID, WorldID: cmd.WorldID,
		SenderHouseholdID: cmd.SenderHouseholdID, ReceiverHouseholdID: cmd.ReceiverHouseholdID,
		OriginLocationID: cmd.OriginLocationID, DestinationLocationID: cmd.DestinationLocationID,
		ResourceType: cmd.ResourceType, QuantityMilli: cmd.QuantityMilli,
		DepartureTick: cmd.DepartureTick, ExpectedArrivalTick: cmd.ExpectedArrivalTick,
		DepartureGameDay: cmd.DepartureGameDay, ExpectedArrivalGameDay: cmd.ExpectedArrivalGameDay,
		TransportCostMilli: cmd.TransportCostMilli, Status: shipmentdomain.StatusPrepared,
	}
	if err := prepared.Validate(); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	dispatched, err := prepared.Transition(shipmentdomain.StatusInTransit)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	return s.Store.CreateShipment(ctx, dispatched)
}

func (s *ShipmentService) ListForHousehold(ctx context.Context, householdID string) ([]port.ShipmentRecord, error) {
	return s.Store.ListHouseholdShipments(ctx, householdID)
}
