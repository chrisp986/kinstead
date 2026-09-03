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

type ShipmentService struct {
	Store port.ShipmentRepository
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
