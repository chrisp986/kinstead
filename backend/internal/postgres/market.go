//go:build postgres

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	marketdomain "game/backend/internal/domain/market"
	shipmentdomain "game/backend/internal/domain/shipment"
)

var (
	ErrMarketStateChanged        = errors.New("market state changed during purchase")
	ErrInvalidMarketParticipants = errors.New("invalid market participants")
)

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

type MarketPurchaseSnapshot struct {
	Offer            marketdomain.Offer
	Buyer            marketdomain.Buyer
	SellerStockMilli marketdomain.QuantityMilli
	CurrentTick      int64
}

func scanMarketOffer(row rowScanner) (marketdomain.Offer, error) {
	var record MarketOfferRecord
	if err := row.Scan(
		&record.ID, &record.WorldID, &record.SellerHouseholdID, &record.OriginLocationID,
		&record.ResourceType, &record.QuantityRemainingMilli, &record.PricePerUnitMilli,
		&record.CreatedTick, &record.ExpiresTick, &record.Status,
	); err != nil {
		return marketdomain.Offer{}, err
	}
	var expires *marketdomain.Tick
	if record.ExpiresTick != nil {
		value := marketdomain.Tick(*record.ExpiresTick)
		expires = &value
	}
	return marketdomain.Offer{
		ID: marketdomain.OfferID(record.ID), WorldID: marketdomain.WorldID(record.WorldID),
		SellerHouseholdID:      marketdomain.HouseholdID(record.SellerHouseholdID),
		OriginLocationID:       marketdomain.LocationID(record.OriginLocationID),
		ResourceType:           marketdomain.ResourceType(record.ResourceType),
		QuantityRemainingMilli: marketdomain.QuantityMilli(record.QuantityRemainingMilli),
		PricePerUnitMilli:      marketdomain.MoneyMilli(record.PricePerUnitMilli),
		CreatedTick:            marketdomain.Tick(record.CreatedTick), ExpiresTick: expires,
		Status: marketdomain.OfferStatus(record.Status),
	}, nil
}

func marketOfferRecord(value marketdomain.Offer) MarketOfferRecord {
	var expires *int64
	if value.ExpiresTick != nil {
		v := int64(*value.ExpiresTick)
		expires = &v
	}
	return MarketOfferRecord{
		ID: string(value.ID), WorldID: string(value.WorldID),
		SellerHouseholdID: string(value.SellerHouseholdID), OriginLocationID: string(value.OriginLocationID),
		ResourceType: string(value.ResourceType), QuantityRemainingMilli: int64(value.QuantityRemainingMilli),
		PricePerUnitMilli: int64(value.PricePerUnitMilli), CreatedTick: int64(value.CreatedTick),
		ExpiresTick: expires, Status: string(value.Status),
	}
}

func (s *Store) LoadMarketPurchase(ctx context.Context, tx pgx.Tx, offerID, buyerHouseholdID string) (MarketPurchaseSnapshot, error) {
	offer, err := scanMarketOffer(tx.QueryRow(ctx, `
        SELECT id::text, world_id::text, seller_household_id::text, origin_location_id::text,
               resource_code, quantity_remaining_milli, price_per_unit_milli,
               created_tick, expires_tick, status
        FROM market_offers
        WHERE id = $1::uuid
        FOR UPDATE
    `, offerID))
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}

	var currentTick int64
	if err := tx.QueryRow(ctx, `
        SELECT current_tick FROM worlds WHERE id = $1::uuid FOR UPDATE
    `, offer.WorldID).Scan(&currentTick); err != nil {
		return MarketPurchaseSnapshot{}, err
	}

	type household struct {
		id, worldID, locationID string
	}
	households := make(map[string]household, 2)
	rows, err := tx.Query(ctx, `
        SELECT id::text, world_id::text, location_id::text
        FROM households
        WHERE id = $1::uuid OR id = $2::uuid
        ORDER BY id
        FOR UPDATE
    `, offer.SellerHouseholdID, buyerHouseholdID)
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	for rows.Next() {
		var value household
		if err := rows.Scan(&value.id, &value.worldID, &value.locationID); err != nil {
			rows.Close()
			return MarketPurchaseSnapshot{}, err
		}
		households[value.id] = value
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	seller, sellerOK := households[string(offer.SellerHouseholdID)]
	buyer, buyerOK := households[buyerHouseholdID]
	if !sellerOK || !buyerOK || seller.worldID != string(offer.WorldID) ||
		buyer.worldID != string(offer.WorldID) || seller.locationID != string(offer.OriginLocationID) {
		return MarketPurchaseSnapshot{}, ErrInvalidMarketParticipants
	}

	sellerStock, err := stockQuantityForUpdate(ctx, tx, seller.id, string(offer.ResourceType))
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	buyerSilver, err := stockQuantityForUpdate(ctx, tx, buyer.id, "silver")
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}

	return MarketPurchaseSnapshot{
		Offer: offer,
		Buyer: marketdomain.Buyer{
			HouseholdID: marketdomain.HouseholdID(buyer.id), WorldID: marketdomain.WorldID(buyer.worldID),
			LocationID: marketdomain.LocationID(buyer.locationID), SilverMilli: marketdomain.MoneyMilli(buyerSilver),
		},
		SellerStockMilli: marketdomain.QuantityMilli(sellerStock),
		CurrentTick:      currentTick,
	}, nil
}

func stockQuantityForUpdate(ctx context.Context, tx pgx.Tx, householdID, resourceType string) (int64, error) {
	var quantity int64
	err := tx.QueryRow(ctx, `
        SELECT quantity_milli FROM resource_stocks
        WHERE household_id = $1::uuid AND resource_code = $2
        FOR UPDATE
    `, householdID, resourceType).Scan(&quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return quantity, err
}

func (s *Store) PersistMarketPurchase(
	ctx context.Context,
	tx pgx.Tx,
	purchase marketdomain.Purchase,
	shipment shipmentdomain.Shipment,
) (MarketOfferRecord, ShipmentRecord, error) {
	if err := purchase.Offer.Validate(); err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	if err := shipment.Validate(); err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	if shipment.Status != shipmentdomain.StatusInTransit ||
		string(shipment.WorldID) != string(purchase.Offer.WorldID) ||
		string(shipment.SenderHouseholdID) != string(purchase.Offer.SellerHouseholdID) ||
		string(shipment.ReceiverHouseholdID) != string(purchase.Buyer.HouseholdID) ||
		string(shipment.OriginLocationID) != string(purchase.Offer.OriginLocationID) ||
		string(shipment.DestinationLocationID) != string(purchase.Buyer.LocationID) ||
		string(shipment.ResourceType) != string(purchase.Offer.ResourceType) ||
		int64(shipment.QuantityMilli) != int64(purchase.QuantityMilli) ||
		int64(shipment.DepartureTick) != int64(purchase.CurrentTick) {
		return MarketOfferRecord{}, ShipmentRecord{}, ErrMarketStateChanged
	}

	buyerTag, err := tx.Exec(ctx, `
        UPDATE resource_stocks
        SET quantity_milli = quantity_milli - $2, updated_at = now()
        WHERE household_id = $1::uuid AND resource_code = 'silver' AND quantity_milli >= $2
    `, purchase.Buyer.HouseholdID, purchase.CostMilli)
	if err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	if buyerTag.RowsAffected() != 1 {
		return MarketOfferRecord{}, ShipmentRecord{}, marketdomain.ErrInsufficientFunds
	}

	sellerTag, err := tx.Exec(ctx, `
        UPDATE resource_stocks
        SET quantity_milli = quantity_milli - $3, updated_at = now()
        WHERE household_id = $1::uuid AND resource_code = $2 AND quantity_milli >= $3
    `, purchase.Offer.SellerHouseholdID, purchase.Offer.ResourceType, purchase.QuantityMilli)
	if err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	if sellerTag.RowsAffected() != 1 {
		return MarketOfferRecord{}, ShipmentRecord{}, marketdomain.ErrInsufficientStock
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO resource_stocks(household_id, resource_code, quantity_milli, updated_at)
        VALUES ($1::uuid, 'silver', $2, now())
        ON CONFLICT (household_id, resource_code)
        DO UPDATE SET quantity_milli = resource_stocks.quantity_milli + EXCLUDED.quantity_milli,
                      updated_at = now()
    `, purchase.Offer.SellerHouseholdID, purchase.CostMilli); err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}

	previousQuantity := purchase.Offer.QuantityRemainingMilli + purchase.QuantityMilli
	updatedOffer, err := scanMarketOffer(tx.QueryRow(ctx, `
        UPDATE market_offers
        SET quantity_remaining_milli = $2, status = $3, updated_at = now()
        WHERE id = $1::uuid AND status = 'active' AND quantity_remaining_milli = $4
        RETURNING id::text, world_id::text, seller_household_id::text, origin_location_id::text,
                  resource_code, quantity_remaining_milli, price_per_unit_milli,
                  created_tick, expires_tick, status
    `, purchase.Offer.ID, purchase.Offer.QuantityRemainingMilli, purchase.Offer.Status, previousQuantity))
	if errors.Is(err, pgx.ErrNoRows) {
		return MarketOfferRecord{}, ShipmentRecord{}, ErrMarketStateChanged
	}
	if err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}

	createdShipment, err := s.insertShipment(ctx, tx, shipment)
	if err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	if err := insertMarketChronicleFacts(ctx, tx, purchase, createdShipment); err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	return marketOfferRecord(updatedOffer), shipmentRecord(createdShipment), nil
}

func insertMarketChronicleFacts(ctx context.Context, tx pgx.Tx, purchase marketdomain.Purchase, shipment shipmentdomain.Shipment) error {
	for _, fact := range []struct {
		householdID, relatedID, entryType string
	}{
		{string(purchase.Buyer.HouseholdID), string(purchase.Offer.SellerHouseholdID), "market_purchase"},
		{string(purchase.Offer.SellerHouseholdID), string(purchase.Buyer.HouseholdID), "market_sale"},
	} {
		if _, err := tx.Exec(ctx, `
            INSERT INTO chronicle_entries(
                household_id, occurred_tick, entry_type, related_household_id,
                related_shipment_id, data
            ) VALUES (
                $1::uuid, $2, $3, $4::uuid, $5::uuid,
                jsonb_build_object(
                    'offer_id', $6::text,
                    'resource_type', $7::text,
                    'quantity_milli', $8::bigint,
                    'cost_milli', $9::bigint
                )
            )
        `, fact.householdID, purchase.CurrentTick, fact.entryType, fact.relatedID, shipment.ID,
			purchase.Offer.ID, purchase.Offer.ResourceType, purchase.QuantityMilli, purchase.CostMilli); err != nil {
			return fmt.Errorf("insert %s chronicle fact: %w", fact.entryType, err)
		}
	}
	return nil
}

func (s *Store) ListActiveMarketOffers(ctx context.Context, worldID string) ([]MarketOfferRecord, error) {
	rows, err := s.Pool.Query(ctx, `
        SELECT o.id::text, o.world_id::text, o.seller_household_id::text, o.origin_location_id::text,
               o.resource_code, o.quantity_remaining_milli, o.price_per_unit_milli,
               o.created_tick, o.expires_tick, o.status
        FROM market_offers o
        JOIN worlds w ON w.id = o.world_id
        WHERE o.world_id = $1::uuid
          AND o.status = 'active'
          AND (o.expires_tick IS NULL OR o.expires_tick >= w.current_tick)
        ORDER BY o.created_tick, o.id
    `, worldID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	offers := make([]MarketOfferRecord, 0)
	for rows.Next() {
		offer, err := scanMarketOffer(rows)
		if err != nil {
			return nil, err
		}
		offers = append(offers, marketOfferRecord(offer))
	}
	return offers, rows.Err()
}
