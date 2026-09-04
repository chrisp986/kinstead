//go:build postgres

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	shipmentdomain "game/backend/internal/domain/shipment"
)

type shipmentFixture struct {
	worldID       string
	senderID      string
	receiverID    string
	originID      string
	destinationID string
	dueIDs        []string
	cancelled     string
}

func TestShipmentArrivalPersistence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	fixture := createShipmentFixture(t, ctx, store)
	t.Cleanup(func() { removeShipmentFixture(t, ctx, store, fixture) })

	created, err := store.CreateShipment(ctx, shipmentdomain.Shipment{
		WorldID:               shipmentdomain.WorldID(fixture.worldID),
		SenderHouseholdID:     shipmentdomain.HouseholdID(fixture.senderID),
		ReceiverHouseholdID:   shipmentdomain.HouseholdID(fixture.receiverID),
		OriginLocationID:      shipmentdomain.LocationID(fixture.originID),
		DestinationLocationID: shipmentdomain.LocationID(fixture.destinationID),
		ResourceType:          "provisions", QuantityMilli: 12_000,
		DepartureTick: 1, ExpectedArrivalTick: 3,
		DepartureGameDay: 7, ExpectedArrivalGameDay: 22,
		Status: shipmentdomain.StatusInTransit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Status != shipmentdomain.StatusInTransit {
		t.Fatalf("created shipment = %+v", created)
	}
	assertStock(t, ctx, store.Pool, fixture.senderID, 38_000)

	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	due, err := store.LoadDueShipments(ctx, tx, fixture.worldID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("tick 1 due shipments = %d, want 0", len(due))
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	// Exercise two arrivals and an idempotent replay, then roll everything back.
	tx, err = store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	due, err = store.LoadDueShipments(ctx, tx, fixture.worldID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 2 {
		t.Fatalf("tick 2 due shipments = %d, want 2", len(due))
	}
	var first shipmentdomain.Shipment
	for i, value := range due {
		arrived, err := value.Arrive(2)
		if err != nil {
			t.Fatal(err)
		}
		persisted, err := store.PersistShipmentArrival(ctx, tx, arrived)
		if err != nil || !persisted {
			t.Fatalf("persist arrival = %v, %v", persisted, err)
		}
		if i == 0 {
			first = arrived
		}
	}
	persisted, err := store.PersistShipmentArrival(ctx, tx, first)
	if err != nil {
		t.Fatal(err)
	}
	if persisted {
		t.Fatal("duplicate arrival was persisted")
	}
	assertStock(t, ctx, tx, fixture.receiverID, 45_000)
	assertShipmentState(t, ctx, tx, fixture.dueIDs[0], "arrived", 2)
	assertShipmentState(t, ctx, tx, fixture.dueIDs[1], "arrived", 2)
	assertShipmentState(t, ctx, tx, fixture.cancelled, "cancelled", 0)
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	// A failed world tick must leave both stock and shipment state untouched.
	assertStock(t, ctx, store.Pool, fixture.receiverID, 10_000)
	assertShipmentState(t, ctx, store.Pool, fixture.dueIDs[0], "in_transit", 0)
	assertShipmentState(t, ctx, store.Pool, fixture.dueIDs[1], "in_transit", 0)

	// Re-run and commit: both independent shipments arrive exactly once.
	tx, err = store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	due, err = store.LoadDueShipments(ctx, tx, fixture.worldID, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range due {
		arrived, err := value.Arrive(2)
		if err != nil {
			t.Fatal(err)
		}
		if persisted, err := store.PersistShipmentArrival(ctx, tx, arrived); err != nil || !persisted {
			t.Fatalf("persist arrival = %v, %v", persisted, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	assertStock(t, ctx, store.Pool, fixture.receiverID, 45_000)
	assertShipmentState(t, ctx, store.Pool, fixture.dueIDs[0], "arrived", 2)
	assertShipmentState(t, ctx, store.Pool, fixture.dueIDs[1], "arrived", 2)

	var facts int
	if err := store.Pool.QueryRow(ctx, `
        SELECT count(*) FROM chronicle_entries
        WHERE household_id = $1::uuid AND entry_type = 'shipment_arrived'
    `, fixture.receiverID).Scan(&facts); err != nil {
		t.Fatal(err)
	}
	if facts != 2 {
		t.Fatalf("shipment arrival facts = %d, want 2", facts)
	}
}

func TestCancelDirectShipmentRefundsReservationExactlyOnce(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	fixture := createShipmentFixture(t, ctx, store)
	t.Cleanup(func() { removeShipmentFixture(t, ctx, store, fixture) })

	prepared := shipmentdomain.Shipment{
		WorldID: shipmentdomain.WorldID(fixture.worldID), SenderHouseholdID: shipmentdomain.HouseholdID(fixture.senderID),
		ReceiverHouseholdID: shipmentdomain.HouseholdID(fixture.receiverID), OriginLocationID: shipmentdomain.LocationID(fixture.originID),
		DestinationLocationID: shipmentdomain.LocationID(fixture.destinationID), ResourceType: "provisions", QuantityMilli: 10_000,
		DepartureTick: 1, ExpectedArrivalTick: 3, DepartureGameDay: 7, ExpectedArrivalGameDay: 22, Status: shipmentdomain.StatusPrepared,
	}
	dispatched, err := prepared.Transition(shipmentdomain.StatusInTransit)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateShipment(ctx, dispatched)
	if err != nil {
		t.Fatal(err)
	}
	assertStock(t, ctx, store.Pool, fixture.senderID, 40_000)

	if _, err := store.CancelShipment(ctx, created.ID, shipmentdomain.HouseholdID(fixture.receiverID)); !errors.Is(err, shipmentdomain.ErrCancellationForbidden) {
		t.Fatalf("other household cancellation error = %v", err)
	}
	assertStock(t, ctx, store.Pool, fixture.senderID, 40_000)

	cancelled, err := store.CancelShipment(ctx, created.ID, shipmentdomain.HouseholdID(fixture.senderID))
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != shipmentdomain.StatusCancelled {
		t.Fatalf("cancelled status = %s", cancelled.Status)
	}
	assertStock(t, ctx, store.Pool, fixture.senderID, 50_000)
	assertShipmentState(t, ctx, store.Pool, string(created.ID), "cancelled", 0)

	replayed, err := store.CancelShipment(ctx, created.ID, shipmentdomain.HouseholdID(fixture.senderID))
	if err != nil || replayed.Status != shipmentdomain.StatusCancelled {
		t.Fatalf("duplicate cancellation = %+v, %v", replayed, err)
	}
	assertStock(t, ctx, store.Pool, fixture.senderID, 50_000)

	var facts int
	if err := store.Pool.QueryRow(ctx, `
		SELECT count(*) FROM chronicle_entries
		WHERE related_shipment_id = $1::uuid AND entry_type = 'shipment_cancelled'
	`, created.ID).Scan(&facts); err != nil {
		t.Fatal(err)
	}
	if facts != 1 {
		t.Fatalf("cancellation facts = %d, want 1", facts)
	}
}

func createShipmentFixture(t *testing.T, ctx context.Context, store *Store) shipmentFixture {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var f shipmentFixture
	if err := tx.QueryRow(ctx, `
        INSERT INTO worlds(name, historical_start_date, current_tick, current_game_day, calendar_remainder, tick_duration_seconds, next_tick_at)
        VALUES ('shipment integration test', DATE '0980-01-01', 1, 7, 7, 3600, $1)
        RETURNING id::text
    `, time.Now().Add(time.Hour)).Scan(&f.worldID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO locations(world_id, name, location_type)
        VALUES ($1::uuid, 'origin', 'farm') RETURNING id::text
    `, f.worldID).Scan(&f.originID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO locations(world_id, name, location_type)
        VALUES ($1::uuid, 'destination', 'farm') RETURNING id::text
    `, f.worldID).Scan(&f.destinationID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO households(world_id, location_id, name, created_tick)
        VALUES ($1::uuid, $2::uuid, 'sender', 0) RETURNING id::text
    `, f.worldID, f.originID).Scan(&f.senderID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO households(world_id, location_id, name, created_tick)
        VALUES ($1::uuid, $2::uuid, 'receiver', 0) RETURNING id::text
    `, f.worldID, f.destinationID).Scan(&f.receiverID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
        INSERT INTO resource_stocks(household_id, resource_code, quantity_milli)
        VALUES ($1::uuid, 'provisions', 50000), ($2::uuid, 'provisions', 10000)
    `, f.senderID, f.receiverID); err != nil {
		t.Fatal(err)
	}

	for _, quantity := range []int64{30_000, 5_000} {
		var id string
		if err := tx.QueryRow(ctx, `
            INSERT INTO shipments(
                world_id, sender_household_id, receiver_household_id,
                origin_location_id, destination_location_id, resource_code,
                quantity_milli, departure_tick, expected_arrival_tick, departure_game_day, expected_arrival_game_day, status
            ) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'provisions',$6,1,2,7,15,'in_transit')
            RETURNING id::text
        `, f.worldID, f.senderID, f.receiverID, f.originID, f.destinationID, quantity).Scan(&id); err != nil {
			t.Fatal(err)
		}
		f.dueIDs = append(f.dueIDs, id)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO shipments(
            world_id, sender_household_id, receiver_household_id,
            origin_location_id, destination_location_id, resource_code,
            quantity_milli, departure_tick, expected_arrival_tick, departure_game_day, expected_arrival_game_day, status
        ) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'provisions',7000,1,2,7,15,'cancelled')
        RETURNING id::text
    `, f.worldID, f.senderID, f.receiverID, f.originID, f.destinationID).Scan(&f.cancelled); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return f
}

func assertStock(t *testing.T, ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, householdID string, want int64) {
	t.Helper()
	var got int64
	if err := query.QueryRow(ctx, `
        SELECT quantity_milli FROM resource_stocks
        WHERE household_id = $1::uuid AND resource_code = 'provisions'
    `, householdID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("receiver provisions = %d, want %d", got, want)
	}
}

func assertShipmentState(t *testing.T, ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, shipmentID, wantStatus string, wantActual int64) {
	t.Helper()
	var status string
	var actual *int64
	if err := query.QueryRow(ctx, `
        SELECT status, actual_arrival_tick FROM shipments WHERE id = $1::uuid
    `, shipmentID).Scan(&status, &actual); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus {
		t.Fatalf("shipment %s status = %q, want %q", shipmentID, status, wantStatus)
	}
	if wantActual == 0 && actual != nil {
		t.Fatalf("shipment %s actual tick = %d, want NULL", shipmentID, *actual)
	}
	if wantActual != 0 && (actual == nil || *actual != wantActual) {
		t.Fatalf("shipment %s actual tick = %v, want %d", shipmentID, actual, wantActual)
	}
}

func removeShipmentFixture(t *testing.T, ctx context.Context, store *Store, f shipmentFixture) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM chronicle_entries WHERE household_id IN (SELECT id FROM households WHERE world_id = $1::uuid)`,
		`DELETE FROM shipments WHERE world_id = $1::uuid`,
		`DELETE FROM households WHERE world_id = $1::uuid`,
		`DELETE FROM locations WHERE world_id = $1::uuid`,
		`DELETE FROM worlds WHERE id = $1::uuid`,
	} {
		if _, err := store.Pool.Exec(ctx, statement, f.worldID); err != nil {
			t.Errorf("fixture cleanup: %v", err)
		}
	}
}
