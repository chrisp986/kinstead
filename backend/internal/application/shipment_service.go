package application

import (
	"context"

	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
)

type CreateShipmentCommand struct {
	ID                    shipmentdomain.ID
	WorldID               shipmentdomain.WorldID
	SenderHouseholdID     shipmentdomain.HouseholdID
	ReceiverHouseholdID   shipmentdomain.HouseholdID
	OriginLocationID      shipmentdomain.LocationID
	DestinationLocationID shipmentdomain.LocationID
	ResourceType          shipmentdomain.ResourceType
	QuantityMilli         shipmentdomain.QuantityMilli
	DepartureTick         shipmentdomain.Tick
	ExpectedArrivalTick   shipmentdomain.Tick
	TransportCostMilli    shipmentdomain.MoneyMilli
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
	return port.ShipmentRecord{
		ID: string(value.ID), WorldID: string(value.WorldID),
		SenderHouseholdID: string(value.SenderHouseholdID), ReceiverHouseholdID: string(value.ReceiverHouseholdID),
		OriginLocationID: string(value.OriginLocationID), DestinationLocationID: string(value.DestinationLocationID),
		ResourceType: string(value.ResourceType), QuantityMilli: int64(value.QuantityMilli),
		DepartureTick: int64(value.DepartureTick), ExpectedArrivalTick: int64(value.ExpectedArrivalTick),
		TransportCostMilli: int64(value.TransportCostMilli), Status: string(value.Status),
	}, nil
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
