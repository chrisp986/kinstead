//go:build postgres

package application

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	marketdomain "game/backend/internal/domain/market"
	"game/backend/internal/postgres"
)

type marketFixture struct {
	worldID        string
	sellerID       string
	buyerIDs       []string
	originID       string
	destinationIDs []string
	offerID        string
}

func TestMarketPurchaseCreatesShipmentAtomically(t *testing.T) {
	ctx, store := openMarketTestStore(t)
	fixture := createMarketFixture(t, ctx, store, 100_000, 100_000, 60_000)
	t.Cleanup(func() { removeMarketFixture(t, ctx, store, fixture.worldID) })
	service := NewMarketService(store)

	result, err := purchaseOfferWithRetry(ctx, service, PurchaseOfferCommand{
		OfferID: fixture.offerID, BuyerHouseholdID: fixture.buyerIDs[0], QuantityMilli: 20_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CostMilli != 31_000 || result.GoodsCostMilli != 30_000 || result.TransportCostMilli != 1_000 || result.Offer.QuantityRemainingMilli != 40_000 || result.Offer.Status != "active" {
		t.Fatalf("purchase result = %+v", result)
	}
	if result.Shipment.Status != "in_transit" || result.Shipment.DepartureTick != 4 ||
		result.Shipment.ExpectedArrivalTick != 6 || result.Shipment.QuantityMilli != 20_000 || result.Shipment.TransportCostMilli != 1_000 {
		t.Fatalf("shipment = %+v", result.Shipment)
	}
	assertMarketStock(t, ctx, store, fixture.buyerIDs[0], "silver", 69_000)
	assertMarketStock(t, ctx, store, fixture.buyerIDs[0], "provisions", 7_000)
	assertMarketStock(t, ctx, store, fixture.sellerID, "provisions", 80_000)
	assertMarketStock(t, ctx, store, fixture.sellerID, "silver", 30_000)
	assertMarketCounts(t, ctx, store, fixture.worldID, 1, 2)

	result, err = purchaseOfferWithRetry(ctx, service, PurchaseOfferCommand{
		OfferID: fixture.offerID, BuyerHouseholdID: fixture.buyerIDs[0], QuantityMilli: 40_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CostMilli != 61_000 || result.Offer.QuantityRemainingMilli != 0 || result.Offer.Status != "filled" {
		t.Fatalf("full purchase result = %+v", result)
	}
	assertMarketStock(t, ctx, store, fixture.buyerIDs[0], "silver", 8_000)
	assertMarketStock(t, ctx, store, fixture.buyerIDs[0], "provisions", 7_000)
	assertMarketStock(t, ctx, store, fixture.sellerID, "provisions", 40_000)
	assertMarketStock(t, ctx, store, fixture.sellerID, "silver", 90_000)
	assertMarketCounts(t, ctx, store, fixture.worldID, 2, 4)
}

func TestMarketPurchaseFailuresRollBack(t *testing.T) {
	ctx, store := openMarketTestStore(t)
	fixture := createMarketFixture(t, ctx, store, 100_000, 1_000, 60_000)
	t.Cleanup(func() { removeMarketFixture(t, ctx, store, fixture.worldID) })
	service := NewMarketService(store)

	_, err := purchaseOfferWithRetry(ctx, service, PurchaseOfferCommand{
		OfferID: fixture.offerID, BuyerHouseholdID: fixture.buyerIDs[0], QuantityMilli: 20_000,
	})
	if !errors.Is(err, marketdomain.ErrInsufficientFunds) {
		t.Fatalf("error = %v, want insufficient funds", err)
	}
	assertMarketStock(t, ctx, store, fixture.buyerIDs[0], "silver", 1_000)
	assertMarketStock(t, ctx, store, fixture.sellerID, "provisions", 100_000)
	assertMarketOffer(t, ctx, store, fixture.offerID, 60_000, "active")
	assertMarketCounts(t, ctx, store, fixture.worldID, 0, 0)

	if _, err := store.Pool.Exec(ctx, `
        UPDATE resource_stocks SET quantity_milli = 100000
        WHERE household_id = $1::uuid AND resource_code = 'silver'
    `, fixture.buyerIDs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `
        UPDATE resource_stocks SET quantity_milli = 10000
        WHERE household_id = $1::uuid AND resource_code = 'provisions'
    `, fixture.sellerID); err != nil {
		t.Fatal(err)
	}
	_, err = purchaseOfferWithRetry(ctx, service, PurchaseOfferCommand{
		OfferID: fixture.offerID, BuyerHouseholdID: fixture.buyerIDs[0], QuantityMilli: 20_000,
	})
	if !errors.Is(err, marketdomain.ErrInsufficientStock) {
		t.Fatalf("error = %v, want insufficient stock", err)
	}
	assertMarketStock(t, ctx, store, fixture.buyerIDs[0], "silver", 100_000)
	assertMarketStock(t, ctx, store, fixture.sellerID, "provisions", 10_000)
	assertMarketOffer(t, ctx, store, fixture.offerID, 60_000, "active")
	assertMarketCounts(t, ctx, store, fixture.worldID, 0, 0)
}

func TestConcurrentMarketPurchasesCannotOversell(t *testing.T) {
	ctx, store := openMarketTestStore(t)
	fixture := createMarketFixture(t, ctx, store, 100_000, 100_000, 60_000)
	t.Cleanup(func() { removeMarketFixture(t, ctx, store, fixture.worldID) })
	service := NewMarketService(store)

	type outcome struct {
		result PurchaseOfferResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	var wg sync.WaitGroup
	for _, buyerID := range fixture.buyerIDs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := purchaseOfferWithRetry(ctx, service, PurchaseOfferCommand{
				OfferID: fixture.offerID, BuyerHouseholdID: buyerID, QuantityMilli: 40_000,
			})
			outcomes <- outcome{result, err}
		}()
	}
	wg.Wait()
	close(outcomes)

	var successes, offerConflicts int
	for result := range outcomes {
		switch {
		case result.err == nil:
			successes++
		case errors.Is(result.err, marketdomain.ErrInsufficientOffer):
			offerConflicts++
		default:
			t.Fatalf("unexpected purchase error: %v", result.err)
		}
	}
	if successes != 1 || offerConflicts != 1 {
		t.Fatalf("successes/conflicts = %d/%d, want 1/1", successes, offerConflicts)
	}
	assertMarketOffer(t, ctx, store, fixture.offerID, 20_000, "active")
	assertMarketStock(t, ctx, store, fixture.sellerID, "provisions", 60_000)
	assertMarketStock(t, ctx, store, fixture.sellerID, "silver", 60_000)
	assertMarketCounts(t, ctx, store, fixture.worldID, 1, 2)
}

func openMarketTestStore(t *testing.T) (context.Context, *postgres.Store) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return ctx, store
}

func purchaseOfferWithRetry(ctx context.Context, service *MarketService, cmd PurchaseOfferCommand) (PurchaseOfferResult, error) {
	var result PurchaseOfferResult
	var err error
	for range 5 {
		result, err = service.PurchaseOffer(ctx, cmd)
		if err == nil || (!strings.Contains(err.Error(), "could not serialize access") && !strings.Contains(err.Error(), "serializable market conflict")) {
			return result, err
		}
	}
	return result, err
}

func createMarketFixture(
	t *testing.T,
	ctx context.Context,
	store *postgres.Store,
	sellerStock, buyerSilver, offerQuantity int64,
) marketFixture {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var fixture marketFixture
	if err := tx.QueryRow(ctx, `
        INSERT INTO worlds(name, historical_start_date, current_tick, tick_duration_seconds, next_tick_at)
        VALUES ('market integration test', DATE '0980-01-01', 4, 3600, now() + interval '1 day')
        RETURNING id::text
    `).Scan(&fixture.worldID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO locations(world_id, name, location_type)
        VALUES ($1::uuid, 'market seller origin', 'farm') RETURNING id::text
    `, fixture.worldID).Scan(&fixture.originID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		var locationID string
		if err := tx.QueryRow(ctx, `
            INSERT INTO locations(world_id, name, location_type)
            VALUES ($1::uuid, $2, 'farm') RETURNING id::text
        `, fixture.worldID, "market buyer destination").Scan(&locationID); err != nil {
			t.Fatal(err)
		}
		fixture.destinationIDs = append(fixture.destinationIDs, locationID)
	}
	for _, locationID := range fixture.destinationIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO location_routes(world_id, origin_location_id, destination_location_id, distance_class)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'local')
		`, fixture.worldID, fixture.originID, locationID); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO households(world_id, location_id, name, created_tick)
        VALUES ($1::uuid, $2::uuid, 'market seller', 0) RETURNING id::text
    `, fixture.worldID, fixture.originID).Scan(&fixture.sellerID); err != nil {
		t.Fatal(err)
	}
	for _, locationID := range fixture.destinationIDs {
		var buyerID string
		if err := tx.QueryRow(ctx, `
            INSERT INTO households(world_id, location_id, name, created_tick)
            VALUES ($1::uuid, $2::uuid, 'market buyer', 0) RETURNING id::text
        `, fixture.worldID, locationID).Scan(&buyerID); err != nil {
			t.Fatal(err)
		}
		fixture.buyerIDs = append(fixture.buyerIDs, buyerID)
	}
	if _, err := tx.Exec(ctx, `
        INSERT INTO resource_stocks(household_id, resource_code, quantity_milli)
        VALUES
            ($1::uuid, 'provisions', $2),
            ($1::uuid, 'silver', 0),
            ($3::uuid, 'silver', $4),
            ($3::uuid, 'provisions', 7000),
            ($5::uuid, 'silver', $4),
            ($5::uuid, 'provisions', 7000)
    `, fixture.sellerID, sellerStock, fixture.buyerIDs[0], buyerSilver, fixture.buyerIDs[1]); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO market_offers(
            world_id, seller_household_id, origin_location_id, resource_code,
            quantity_remaining_milli, price_per_unit_milli, created_tick, status
        ) VALUES ($1::uuid,$2::uuid,$3::uuid,'provisions',$4,1500,4,'active')
        RETURNING id::text
    `, fixture.worldID, fixture.sellerID, fixture.originID, offerQuantity).Scan(&fixture.offerID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func assertMarketStock(t *testing.T, ctx context.Context, store *postgres.Store, householdID, resourceType string, want int64) {
	t.Helper()
	var got int64
	if err := store.Pool.QueryRow(ctx, `
        SELECT quantity_milli FROM resource_stocks
        WHERE household_id = $1::uuid AND resource_code = $2
    `, householdID, resourceType).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s %s = %d, want %d", householdID, resourceType, got, want)
	}
}

func assertMarketOffer(t *testing.T, ctx context.Context, store *postgres.Store, offerID string, wantQuantity int64, wantStatus string) {
	t.Helper()
	var quantity int64
	var status string
	if err := store.Pool.QueryRow(ctx, `
        SELECT quantity_remaining_milli, status FROM market_offers WHERE id = $1::uuid
    `, offerID).Scan(&quantity, &status); err != nil {
		t.Fatal(err)
	}
	if quantity != wantQuantity || status != wantStatus {
		t.Fatalf("offer quantity/status = %d/%s, want %d/%s", quantity, status, wantQuantity, wantStatus)
	}
}

func assertMarketCounts(t *testing.T, ctx context.Context, store *postgres.Store, worldID string, wantShipments, wantFacts int) {
	t.Helper()
	var shipments, facts int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM shipments WHERE world_id = $1::uuid`, worldID).Scan(&shipments); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `
        SELECT count(*) FROM chronicle_entries
        WHERE household_id IN (SELECT id FROM households WHERE world_id = $1::uuid)
    `, worldID).Scan(&facts); err != nil {
		t.Fatal(err)
	}
	if shipments != wantShipments || facts != wantFacts {
		t.Fatalf("shipments/facts = %d/%d, want %d/%d", shipments, facts, wantShipments, wantFacts)
	}
}

func removeMarketFixture(t *testing.T, ctx context.Context, store *postgres.Store, worldID string) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM chronicle_entries WHERE household_id IN (SELECT id FROM households WHERE world_id = $1::uuid)`,
		`DELETE FROM shipments WHERE world_id = $1::uuid`,
		`DELETE FROM market_offers WHERE world_id = $1::uuid`,
		`DELETE FROM households WHERE world_id = $1::uuid`,
		`DELETE FROM locations WHERE world_id = $1::uuid`,
		`DELETE FROM worlds WHERE id = $1::uuid`,
	} {
		if _, err := store.Pool.Exec(ctx, statement, worldID); err != nil {
			t.Errorf("fixture cleanup: %v", err)
		}
	}
}
