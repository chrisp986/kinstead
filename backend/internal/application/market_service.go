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

type MarketQuote struct {
	GoodsCostMilli         int64 `json:"goods_cost_milli"`
	TransportCostMilli     int64 `json:"transport_cost_milli"`
	TotalCostMilli         int64 `json:"total_cost_milli"`
	RemainingSilverMilli   int64 `json:"remaining_silver_milli"`
	ExpectedArrivalGameDay int64 `json:"expected_arrival_game_day"`
	TravelTicks            int64 `json:"travel_ticks"`
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

	expectedArrivalGameDay, err := gameDayAfterTicks(snapshot.CurrentGameDay, snapshot.CalendarRemainder, snapshot.GameDaysPerTickNum, snapshot.GameDaysPerTickDen, int64(snapshot.Route.TravelTicks))
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
		ExpectedArrivalGameDay: shipmentdomain.GameDay(expectedArrivalGameDay),
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

// QuoteOffer evaluates the same authoritative market rules as a purchase and
// always rolls its transaction back, so it reserves neither stock nor silver.
func (s *MarketService) QuoteOffer(ctx context.Context, cmd PurchaseOfferCommand) (MarketQuote, error) {
	if cmd.QuantityMilli <= 0 {
		return MarketQuote{}, marketdomain.ErrInvalidQuantity
	}
	tx, err := s.Store.BeginMarketPurchase(ctx)
	if err != nil {
		return MarketQuote{}, err
	}
	defer tx.Rollback(ctx)
	snapshot, err := tx.Load(ctx, cmd.OfferID, cmd.BuyerHouseholdID)
	if err != nil {
		return MarketQuote{}, err
	}
	purchase, err := marketdomain.EvaluatePurchase(snapshot.Offer, snapshot.Buyer, snapshot.Route, snapshot.SellerStockMilli, marketdomain.QuantityMilli(cmd.QuantityMilli), marketdomain.Tick(snapshot.CurrentTick))
	if err != nil {
		return MarketQuote{}, err
	}
	arrivalDay, err := gameDayAfterTicks(snapshot.CurrentGameDay, snapshot.CalendarRemainder, snapshot.GameDaysPerTickNum, snapshot.GameDaysPerTickDen, int64(snapshot.Route.TravelTicks))
	if err != nil {
		return MarketQuote{}, err
	}
	return MarketQuote{
		GoodsCostMilli: int64(purchase.GoodsCostMilli), TransportCostMilli: int64(purchase.TransportCostMilli),
		TotalCostMilli: int64(purchase.TotalCostMilli), RemainingSilverMilli: int64(snapshot.Buyer.SilverMilli - purchase.TotalCostMilli),
		ExpectedArrivalGameDay: arrivalDay, TravelTicks: int64(snapshot.Route.TravelTicks),
	}, nil
}

func (s *MarketService) ListActiveOffers(ctx context.Context, worldID string) ([]port.MarketOfferRecord, error) {
	return s.Store.ListActiveMarketOffers(ctx, worldID)
}
