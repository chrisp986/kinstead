//go:build postgres

package application

import (
	"context"
	"os"
	"strings"
	"testing"

	"game/backend/internal/postgres"
)

func TestTickProcessorDeliversShipmentInCanonicalOrder(t *testing.T) {
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

	worldID, receiverID, shipmentID := createTickShipmentFixture(t, ctx, store)
	t.Cleanup(func() { removeTickShipmentFixture(t, ctx, store, worldID) })
	processor := NewTickProcessor(store)

	processed, err := processDueWorldWithRetry(ctx, processor)
	if err != nil || !processed {
		t.Fatalf("process tick 1 = %v, %v", processed, err)
	}
	assertTickShipment(t, ctx, store, worldID, receiverID, shipmentID, 1, 5_100, "in_transit", nil)

	if _, err := store.Pool.Exec(ctx, `UPDATE worlds SET next_tick_at = now() - interval '1 day' WHERE id = $1::uuid`, worldID); err != nil {
		t.Fatal(err)
	}
	processed, err = processDueWorldWithRetry(ctx, processor)
	if err != nil || !processed {
		t.Fatalf("process tick 2 = %v, %v", processed, err)
	}
	arrivalTick := int64(2)
	// 10,000 - 4,900 + 30,000 - 4,900: arrival is credited before tick-2 consumption.
	assertTickShipment(t, ctx, store, worldID, receiverID, shipmentID, 2, 30_200, "arrived", &arrivalTick)
}

func processDueWorldWithRetry(ctx context.Context, processor *TickProcessor) (bool, error) {
	var processed bool
	var err error
	for range 5 {
		processed, err = processor.ProcessOneDueWorld(ctx)
		if err == nil || !strings.Contains(err.Error(), "could not serialize access") {
			return processed, err
		}
	}
	return processed, err
}

func createTickShipmentFixture(t *testing.T, ctx context.Context, store *postgres.Store) (string, string, string) {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var worldID string
	if err := tx.QueryRow(ctx, `
        INSERT INTO worlds(name, historical_start_date, current_tick, tick_duration_seconds, next_tick_at)
        VALUES ('tick shipment integration test', DATE '0980-01-01', 0, 172800, now() - interval '1 day')
        RETURNING id::text
    `).Scan(&worldID); err != nil {
		t.Fatal(err)
	}
	var originID, destinationID string
	if err := tx.QueryRow(ctx, `
        INSERT INTO locations(world_id, name, location_type)
        VALUES ($1::uuid, 'tick origin', 'farm') RETURNING id::text
    `, worldID).Scan(&originID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO locations(world_id, name, location_type)
        VALUES ($1::uuid, 'tick destination', 'farm') RETURNING id::text
    `, worldID).Scan(&destinationID); err != nil {
		t.Fatal(err)
	}
	var senderID, receiverID string
	if err := tx.QueryRow(ctx, `
        INSERT INTO households(world_id, location_id, name, created_tick)
        VALUES ($1::uuid, $2::uuid, 'tick sender', 0) RETURNING id::text
    `, worldID, originID).Scan(&senderID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO households(world_id, location_id, name, created_tick)
        VALUES ($1::uuid, $2::uuid, 'tick receiver', 0) RETURNING id::text
    `, worldID, destinationID).Scan(&receiverID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
        INSERT INTO resource_stocks(household_id, resource_code, quantity_milli)
        VALUES ($1::uuid, 'provisions', 10000)
    `, receiverID); err != nil {
		t.Fatal(err)
	}
	var shipmentID string
	if err := tx.QueryRow(ctx, `
        INSERT INTO shipments(
            world_id, sender_household_id, receiver_household_id,
            origin_location_id, destination_location_id, resource_code,
            quantity_milli, departure_tick, expected_arrival_tick, status
        ) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'provisions',30000,0,2,'in_transit')
        RETURNING id::text
    `, worldID, senderID, receiverID, originID, destinationID).Scan(&shipmentID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return worldID, receiverID, shipmentID
}

func assertTickShipment(
	t *testing.T,
	ctx context.Context,
	store *postgres.Store,
	worldID, receiverID, shipmentID string,
	wantWorldTick, wantStock int64,
	wantStatus string,
	wantArrival *int64,
) {
	t.Helper()
	var worldTick, stock int64
	var status string
	var arrival *int64
	if err := store.Pool.QueryRow(ctx, `SELECT current_tick FROM worlds WHERE id = $1::uuid`, worldID).Scan(&worldTick); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `
        SELECT quantity_milli FROM resource_stocks
        WHERE household_id = $1::uuid AND resource_code = 'provisions'
    `, receiverID).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `
        SELECT status, actual_arrival_tick FROM shipments WHERE id = $1::uuid
    `, shipmentID).Scan(&status, &arrival); err != nil {
		t.Fatal(err)
	}
	if worldTick != wantWorldTick || stock != wantStock || status != wantStatus {
		t.Fatalf("tick/stock/status = %d/%d/%s, want %d/%d/%s", worldTick, stock, status, wantWorldTick, wantStock, wantStatus)
	}
	if wantArrival == nil && arrival != nil {
		t.Fatalf("actual arrival = %d, want NULL", *arrival)
	}
	if wantArrival != nil && (arrival == nil || *arrival != *wantArrival) {
		t.Fatalf("actual arrival = %v, want %d", arrival, *wantArrival)
	}
}

func removeTickShipmentFixture(t *testing.T, ctx context.Context, store *postgres.Store, worldID string) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM chronicle_entries WHERE household_id IN (SELECT id FROM households WHERE world_id = $1::uuid)`,
		`DELETE FROM shipments WHERE world_id = $1::uuid`,
		`DELETE FROM households WHERE world_id = $1::uuid`,
		`DELETE FROM locations WHERE world_id = $1::uuid`,
		`DELETE FROM worlds WHERE id = $1::uuid`,
	} {
		if _, err := store.Pool.Exec(ctx, statement, worldID); err != nil {
			t.Errorf("fixture cleanup: %v", err)
		}
	}
}
