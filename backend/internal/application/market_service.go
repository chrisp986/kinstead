//go:build postgres

package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	marketdomain "game/backend/internal/domain/market"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/postgres"
)

const defaultMarketTravelTicks marketdomain.Tick = 2

type PurchaseOfferCommand struct {
	OfferID          string
	BuyerHouseholdID string
	QuantityMilli    int64
}

type PurchaseOfferResult struct {
	CostMilli int64                      `json:"cost_milli"`
	Offer     postgres.MarketOfferRecord `json:"offer"`
	Shipment  postgres.ShipmentRecord    `json:"shipment"`
}

type MarketService struct {
	Store       *postgres.Store
	TravelTicks marketdomain.Tick
}

func NewMarketService(store *postgres.Store) *MarketService {
	return &MarketService{Store: store, TravelTicks: defaultMarketTravelTicks}
}

func (s *MarketService) PurchaseOffer(ctx context.Context, cmd PurchaseOfferCommand) (PurchaseOfferResult, error) {
	if cmd.QuantityMilli <= 0 {
		return PurchaseOfferResult{}, marketdomain.ErrInvalidQuantity
	}
	tx, err := s.Store.Begin(ctx)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	defer tx.Rollback(ctx)

	snapshot, err := s.Store.LoadMarketPurchase(ctx, tx, cmd.OfferID, cmd.BuyerHouseholdID)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	purchase, err := marketdomain.EvaluatePurchase(
		snapshot.Offer,
		snapshot.Buyer,
		snapshot.SellerStockMilli,
		marketdomain.QuantityMilli(cmd.QuantityMilli),
		marketdomain.Tick(snapshot.CurrentTick),
	)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	arrivalTick, err := marketdomain.ArrivalTick(purchase.CurrentTick, s.TravelTicks)
	if err != nil {
		return PurchaseOfferResult{}, err
	}

	prepared := shipmentdomain.Shipment{
		WorldID:               shipmentdomain.WorldID(snapshot.Offer.WorldID),
		SenderHouseholdID:     shipmentdomain.HouseholdID(snapshot.Offer.SellerHouseholdID),
		ReceiverHouseholdID:   shipmentdomain.HouseholdID(snapshot.Buyer.HouseholdID),
		OriginLocationID:      shipmentdomain.LocationID(snapshot.Offer.OriginLocationID),
		DestinationLocationID: shipmentdomain.LocationID(snapshot.Buyer.LocationID),
		ResourceType:          shipmentdomain.ResourceType(snapshot.Offer.ResourceType),
		QuantityMilli:         shipmentdomain.QuantityMilli(purchase.QuantityMilli),
		DepartureTick:         shipmentdomain.Tick(purchase.CurrentTick),
		ExpectedArrivalTick:   shipmentdomain.Tick(arrivalTick),
		Status:                shipmentdomain.StatusPrepared,
	}
	if err := prepared.Validate(); err != nil {
		return PurchaseOfferResult{}, err
	}
	shipment, err := prepared.Transition(shipmentdomain.StatusInTransit)
	if err != nil {
		return PurchaseOfferResult{}, err
	}

	offerRecord, shipmentRecord, err := s.Store.PersistMarketPurchase(ctx, tx, purchase, shipment)
	if err != nil {
		return PurchaseOfferResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxCommitRollback) {
			return PurchaseOfferResult{}, fmt.Errorf("serializable market conflict: %w", err)
		}
		return PurchaseOfferResult{}, err
	}
	return PurchaseOfferResult{CostMilli: int64(purchase.CostMilli), Offer: offerRecord, Shipment: shipmentRecord}, nil
}

func (s *MarketService) ListActiveOffers(ctx context.Context, worldID string) ([]postgres.MarketOfferRecord, error) {
	return s.Store.ListActiveMarketOffers(ctx, worldID)
}
