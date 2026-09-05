//go:build postgres

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	marketdomain "game/backend/internal/domain/market"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
	sqlcdb "game/backend/internal/postgres/db"
)

var (
	ErrMarketStateChanged        = errors.New("market state changed during purchase")
	ErrInvalidMarketParticipants = errors.New("invalid market participants")
)

type MarketOfferRecord = port.MarketOfferRecord

type MarketPurchaseSnapshot = port.MarketPurchaseSnapshot

type marketPurchaseTx struct {
	store *Store
	tx    pgx.Tx
}

func (s *Store) BeginMarketPurchase(ctx context.Context) (port.MarketPurchaseTransaction, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &marketPurchaseTx{store: s, tx: tx}, nil
}

func (t *marketPurchaseTx) Load(ctx context.Context, offerID, buyerHouseholdID string) (port.MarketPurchaseSnapshot, error) {
	return t.store.LoadMarketPurchase(ctx, t.tx, offerID, buyerHouseholdID)
}

func (t *marketPurchaseTx) Persist(ctx context.Context, purchase marketdomain.Purchase, shipment shipmentdomain.Shipment) (port.MarketOfferRecord, port.ShipmentRecord, error) {
	return t.store.PersistMarketPurchase(ctx, t.tx, purchase, shipment)
}

func (t *marketPurchaseTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *marketPurchaseTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

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
	offerUUID, err := uuidParam(offerID)
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	locked, err := sqlcdb.New(tx).LockMarketOffer(ctx, offerUUID)
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	offer := marketOfferFromSQLC(locked.ID, locked.WorldID, locked.SellerHouseholdID, locked.OriginLocationID,
		locked.ResourceCode, locked.QuantityRemainingMilli, locked.PricePerUnitMilli, locked.CreatedTick, locked.ExpiresTick, locked.Status)

	var currentTick, currentGameDay, calendarRemainder, rateNumerator, rateDenominator int64
	if err := tx.QueryRow(ctx, `
	        SELECT current_tick, current_game_day, calendar_remainder, game_days_per_tick_num, game_days_per_tick_den
        FROM worlds WHERE id = $1::uuid FOR UPDATE
	`, offer.WorldID).Scan(&currentTick, &currentGameDay, &calendarRemainder, &rateNumerator, &rateDenominator); err != nil {
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
	worldUUID, err := uuidParam(string(offer.WorldID))
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	originUUID, err := uuidParam(string(offer.OriginLocationID))
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	destinationUUID, err := uuidParam(buyer.locationID)
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}
	distanceValue, err := sqlcdb.New(tx).GetRouteDistance(ctx, sqlcdb.GetRouteDistanceParams{
		Column1: worldUUID, Column2: originUUID, Column3: destinationUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MarketPurchaseSnapshot{}, marketdomain.ErrRouteUnavailable
		}
		return MarketPurchaseSnapshot{}, err
	}
	distanceClass := marketdomain.DistanceClass(distanceValue)
	route, err := marketdomain.RouteForDistance(offer.WorldID, offer.OriginLocationID, marketdomain.LocationID(buyer.locationID), distanceClass)
	if err != nil {
		return MarketPurchaseSnapshot{}, err
	}

	return MarketPurchaseSnapshot{
		Offer: offer,
		Buyer: marketdomain.Buyer{
			HouseholdID: marketdomain.HouseholdID(buyer.id), WorldID: marketdomain.WorldID(buyer.worldID),
			LocationID: marketdomain.LocationID(buyer.locationID), SilverMilli: marketdomain.MoneyMilli(buyerSilver),
		},
		Route:            route,
		SellerStockMilli: marketdomain.QuantityMilli(sellerStock),
		CurrentTick:      currentTick,
		CurrentGameDay:   currentGameDay, CalendarRemainder: calendarRemainder,
		GameDaysPerTickNum: rateNumerator, GameDaysPerTickDen: rateDenominator,
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
	`, purchase.Buyer.HouseholdID, purchase.TotalCostMilli)
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
	`, purchase.Offer.SellerHouseholdID, purchase.GoodsCostMilli); err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}

	previousQuantity := purchase.Offer.QuantityRemainingMilli + purchase.QuantityMilli
	offerUUID, err := uuidParam(string(purchase.Offer.ID))
	if err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	updated, err := sqlcdb.New(tx).UpdateMarketOfferAfterPurchase(ctx, sqlcdb.UpdateMarketOfferAfterPurchaseParams{
		Column1: offerUUID, QuantityRemainingMilli: int64(purchase.Offer.QuantityRemainingMilli),
		Status: string(purchase.Offer.Status), QuantityRemainingMilli_2: int64(previousQuantity),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return MarketOfferRecord{}, ShipmentRecord{}, ErrMarketStateChanged
	}
	if err != nil {
		return MarketOfferRecord{}, ShipmentRecord{}, err
	}
	updatedOffer := marketOfferFromSQLC(updated.ID, updated.WorldID, updated.SellerHouseholdID, updated.OriginLocationID,
		updated.ResourceCode, updated.QuantityRemainingMilli, updated.PricePerUnitMilli, updated.CreatedTick, updated.ExpiresTick, updated.Status)

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
                household_id, occurred_tick, occurred_game_day, entry_type, related_household_id,
                related_shipment_id, data
            ) VALUES (
                $1::uuid, $2, $12, $3, $4::uuid, $5::uuid,
                jsonb_build_object(
                    'offer_id', $6::text,
                    'resource_type', $7::text,
                    'quantity_milli', $8::bigint,
					'goods_cost_milli', $9::bigint,
					'transport_cost_milli', $10::bigint,
					'total_cost_milli', $11::bigint
                )
            )
        `, fact.householdID, purchase.CurrentTick, fact.entryType, fact.relatedID, shipment.ID,
			purchase.Offer.ID, purchase.Offer.ResourceType, purchase.QuantityMilli,
			purchase.GoodsCostMilli, purchase.TransportCostMilli, purchase.TotalCostMilli,
			shipment.DepartureGameDay); err != nil {
			return fmt.Errorf("insert %s chronicle fact: %w", fact.entryType, err)
		}
	}
	return nil
}

func (s *Store) ListActiveMarketOffers(ctx context.Context, worldID string) ([]MarketOfferRecord, error) {
	id, err := uuidParam(worldID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcdb.New(s.Pool).ListActiveMarketOffers(ctx, id)
	if err != nil {
		return nil, err
	}
	offers := make([]MarketOfferRecord, 0, len(rows))
	for _, row := range rows {
		offer := marketOfferFromSQLC(row.ID, row.WorldID, row.SellerHouseholdID, row.OriginLocationID,
			row.ResourceCode, row.QuantityRemainingMilli, row.PricePerUnitMilli, row.CreatedTick, row.ExpiresTick, row.Status)
		offers = append(offers, marketOfferRecord(offer))
	}
	return offers, nil
}

func marketOfferFromSQLC(id, worldID, sellerID, originID, resource string, quantity, price, created int64, expiresValue pgtype.Int8, status string) marketdomain.Offer {
	var expires *marketdomain.Tick
	if expiresValue.Valid {
		value := marketdomain.Tick(expiresValue.Int64)
		expires = &value
	}
	return marketdomain.Offer{
		ID: marketdomain.OfferID(id), WorldID: marketdomain.WorldID(worldID), SellerHouseholdID: marketdomain.HouseholdID(sellerID),
		OriginLocationID: marketdomain.LocationID(originID), ResourceType: marketdomain.ResourceType(resource),
		QuantityRemainingMilli: marketdomain.QuantityMilli(quantity), PricePerUnitMilli: marketdomain.MoneyMilli(price),
		CreatedTick: marketdomain.Tick(created), ExpiresTick: expires, Status: marketdomain.OfferStatus(status),
	}
}
