package application

import (
	"context"
	"fmt"

	marketdomain "game/backend/internal/domain/market"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
)

type PurchaseOfferCommand struct {
	OfferID          string
	BuyerHouseholdID string
	QuantityMilli    int64
}

type PurchaseOfferResult struct {
	CostMilli          int64                  `json:"cost_milli"`
	GoodsCostMilli     int64                  `json:"goods_cost_milli"`
	TransportCostMilli int64                  `json:"transport_cost_milli"`
	Offer              port.MarketOfferRecord `json:"offer"`
	Shipment           port.ShipmentRecord    `json:"shipment"`
}

type MarketService struct {
	Store port.MarketRepository
}

func NewMarketService(store port.MarketRepository) *MarketService {
	return &MarketService{Store: store}
}

func (s *MarketService) PurchaseOffer(ctx context.Context, cmd PurchaseOfferCommand) (PurchaseOfferResult, error) {
	if cmd.QuantityMilli <= 0 {
		return PurchaseOfferResult{}, marketdomain.ErrInvalidQuantity
	}
	tx, err := s.Store.BeginMarketPurchase(ctx)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	defer tx.Rollback(ctx)

	snapshot, err := tx.Load(ctx, cmd.OfferID, cmd.BuyerHouseholdID)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	purchase, err := marketdomain.EvaluatePurchase(
		snapshot.Offer,
		snapshot.Buyer,
		snapshot.Route,
		snapshot.SellerStockMilli,
		marketdomain.QuantityMilli(cmd.QuantityMilli),
		marketdomain.Tick(snapshot.CurrentTick),
	)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	arrivalTick, err := marketdomain.ArrivalTick(purchase.CurrentTick, snapshot.Route.TravelTicks)
	if err != nil {
		return PurchaseOfferResult{}, err
	}

	prepared := shipmentdomain.Shipment{
		WorldID:                shipmentdomain.WorldID(snapshot.Offer.WorldID),
		SenderHouseholdID:      shipmentdomain.HouseholdID(snapshot.Offer.SellerHouseholdID),
		ReceiverHouseholdID:    shipmentdomain.HouseholdID(snapshot.Buyer.HouseholdID),
		OriginLocationID:       shipmentdomain.LocationID(snapshot.Offer.OriginLocationID),
		DestinationLocationID:  shipmentdomain.LocationID(snapshot.Buyer.LocationID),
		ResourceType:           shipmentdomain.ResourceType(snapshot.Offer.ResourceType),
		QuantityMilli:          shipmentdomain.QuantityMilli(purchase.QuantityMilli),
		DepartureTick:          shipmentdomain.Tick(purchase.CurrentTick),
		ExpectedArrivalTick:    shipmentdomain.Tick(arrivalTick),
		DepartureGameDay:       shipmentdomain.GameDay(snapshot.CurrentGameDay),
		ExpectedArrivalGameDay: shipmentdomain.GameDay(snapshot.CurrentGameDay + travelDays(int64(snapshot.Route.TravelTicks), snapshot.GameDaysPerTickNum, snapshot.GameDaysPerTickDen)),
		TransportCostMilli:     shipmentdomain.MoneyMilli(purchase.TransportCostMilli),
		Status:                 shipmentdomain.StatusPrepared,
	}
	if err := prepared.Validate(); err != nil {
		return PurchaseOfferResult{}, err
	}
	shipment, err := prepared.Transition(shipmentdomain.StatusInTransit)
	if err != nil {
		return PurchaseOfferResult{}, err
	}

	offerRecord, shipmentRecord, err := tx.Persist(ctx, purchase, shipment)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PurchaseOfferResult{}, fmt.Errorf("commit market purchase: %w", err)
	}
	return PurchaseOfferResult{CostMilli: int64(purchase.TotalCostMilli), GoodsCostMilli: int64(purchase.GoodsCostMilli), TransportCostMilli: int64(purchase.TransportCostMilli), Offer: offerRecord, Shipment: shipmentRecord}, nil
}

func travelDays(ticks int64, numerator, denominator int64) int64 {
	if ticks <= 0 || numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (ticks*numerator + denominator - 1) / denominator
}

func (s *MarketService) ListActiveOffers(ctx context.Context, worldID string) ([]port.MarketOfferRecord, error) {
	return s.Store.ListActiveMarketOffers(ctx, worldID)
}
