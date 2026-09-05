//go:build postgres

package application

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

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

	worldID, receiverID, shipmentID, contractID := createTickShipmentFixture(t, ctx, store)
	t.Cleanup(func() { removeTickShipmentFixture(t, ctx, store, worldID) })
	processor := NewTickProcessor(store)

	processed, err := processDueWorldWithRetry(ctx, processor)
	if err != nil || !processed {
		t.Fatalf("process tick 1 = %v, %v", processed, err)
	}
	assertTickShipment(t, ctx, store, worldID, receiverID, shipmentID, 1, 5_100, "in_transit", nil)
	assertTickContract(t, ctx, store, contractID, "dispatched", "pending", "dispatched", nil, 0, 0)

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
	assertTickContract(t, ctx, store, contractID, "fulfilled", "pending", "dispatched", nil, 1, 2)

	for _, expectation := range []struct {
		tick       int64
		status     string
		eventCount int
		trust      int
	}{{3, "late", 1, 2}, {4, "late", 1, 2}, {5, "broken", 3, -14}} {
		if _, err := store.Pool.Exec(ctx, `UPDATE worlds SET next_tick_at = now() - interval '1 day' WHERE id = $1::uuid`, worldID); err != nil {
			t.Fatal(err)
		}
		processed, err = processDueWorldWithRetry(ctx, processor)
		if err != nil || !processed {
			t.Fatalf("process tick %d = %v, %v", expectation.tick, processed, err)
		}
		assertTickContract(t, ctx, store, contractID, "fulfilled", expectation.status, expectation.status, nil, expectation.eventCount, expectation.trust)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE worlds SET next_tick_at = now() - interval '1 day' WHERE id = $1::uuid`, worldID); err != nil {
		t.Fatal(err)
	}
	processed, err = processDueWorldWithRetry(ctx, processor)
	if err != nil || !processed {
		t.Fatalf("process tick 6 = %v, %v", processed, err)
	}
	delayedArrivalTick := int64(6)
	assertTickContract(t, ctx, store, contractID, "fulfilled", "broken", "broken", &delayedArrivalTick, 3, -14)
	relationships, err := NewRelationshipService(store).ListForHousehold(ctx, receiverID)
	if err != nil || len(relationships) != 1 || relationships[0].Standing != "neutral" || relationships[0].Trust != -14 || len(relationships[0].Events) != 3 {
		t.Fatalf("relationship projection = %+v, %v", relationships, err)
	}
}

func TestTickProcessorAppliesContractTrustByFinalOutcome(t *testing.T) {
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
	fixture := createTrustTickFixture(t, ctx, store)
	t.Cleanup(func() { removeTrustTickFixture(t, ctx, store, fixture.worldID) })
	processor := NewTickProcessor(store)

	want := map[int64]struct {
		trust  int
		events int
	}{
		2: {2, 1},
		3: {4, 2}, // two consecutive on-time fulfillments: +2 +2
		4: {4, 2}, // overdue but not final: no event
		5: {3, 3}, // one tick late: -1
		6: {3, 3},
		7: {1, 4}, // two ticks late: -2
		8: {1, 4},
		9: {-7, 5}, // broken at 3+ ticks late: -8
	}
	for tick := int64(1); tick <= 9; tick++ {
		if tick > 1 {
			if _, err := store.Pool.Exec(ctx, `UPDATE worlds SET next_tick_at = now() - interval '1 day' WHERE id = $1::uuid`, fixture.worldID); err != nil {
				t.Fatal(err)
			}
		}
		processed, err := processDueWorldWithRetry(ctx, processor)
		if err != nil || !processed {
			t.Fatalf("process tick %d = %v, %v", tick, processed, err)
		}
		if expected, ok := want[tick]; ok {
			var trust, events int
			if err := store.Pool.QueryRow(ctx, `SELECT trust FROM relationships WHERE source_household_id = $1::uuid AND target_household_id = $2::uuid`, fixture.creditorID, fixture.debtorID).Scan(&trust); expected.events == 0 && err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					t.Fatal(err)
				}
				trust = 0
			} else if err != nil {
				t.Fatal(err)
			}
			if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM relationship_events WHERE world_id = $1::uuid`, fixture.worldID).Scan(&events); err != nil {
				t.Fatal(err)
			}
			if trust != expected.trust || events != expected.events {
				t.Fatalf("tick %d trust/events = %d/%d, want %d/%d", tick, trust, events, expected.trust, expected.events)
			}
		}
	}

	var reverseCount int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM relationships WHERE source_household_id = $1::uuid AND target_household_id = $2::uuid`, fixture.debtorID, fixture.creditorID).Scan(&reverseCount); err != nil {
		t.Fatal(err)
	}
	if reverseCount != 0 {
		t.Fatalf("reverse relationship count = %d, want 0", reverseCount)
	}
	var deltas []int
	rows, err := store.Pool.Query(ctx, `SELECT trust_delta FROM relationship_events WHERE world_id = $1::uuid ORDER BY occurred_tick, id`, fixture.worldID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var delta int
		if err := rows.Scan(&delta); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		deltas = append(deltas, delta)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantDeltas := []int{2, 2, -1, -2, -8}
	if !reflect.DeepEqual(deltas, wantDeltas) {
		t.Fatalf("relationship trust deltas = %v, want %v", deltas, wantDeltas)
	}
}

type trustTickFixture struct {
	worldID    string
	debtorID   string
	creditorID string
}

func createTrustTickFixture(t *testing.T, ctx context.Context, store *postgres.Store) trustTickFixture {
	t.Helper()
	var fixture trustTickFixture
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `
		INSERT INTO worlds(name, historical_start_date, current_tick, current_game_day, calendar_remainder, tick_duration_seconds, next_tick_at)
		VALUES ('contract trust integration test', DATE '0980-01-01', 0, 0, 0, 172800, now() - interval '1 day')
		RETURNING id::text
	`).Scan(&fixture.worldID); err != nil {
		t.Fatal(err)
	}
	var originID, destinationID string
	if err := tx.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'trust origin', 'farm') RETURNING id::text`, fixture.worldID).Scan(&originID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'trust destination', 'farm') RETURNING id::text`, fixture.worldID).Scan(&destinationID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'trust debtor', 0) RETURNING id::text`, fixture.worldID, originID).Scan(&fixture.debtorID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'trust creditor', 0) RETURNING id::text`, fixture.worldID, destinationID).Scan(&fixture.creditorID); err != nil {
		t.Fatal(err)
	}
	for _, householdID := range []string{fixture.debtorID, fixture.creditorID} {
		if _, err := tx.Exec(ctx, `INSERT INTO resource_stocks(household_id, resource_code, quantity_milli) VALUES ($1::uuid, 'provisions', 100000)`, householdID); err != nil {
			t.Fatal(err)
		}
	}
	var contractID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO contracts(world_id, party_a_household_id, party_b_household_id, starts_tick, ends_tick, interval_ticks, start_game_day, end_game_day, interval_days, game_day_schedule, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 2, 6, 1, 15, 45, 1, false, 'active')
		RETURNING id::text
	`, fixture.worldID, fixture.debtorID, fixture.creditorID).Scan(&contractID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO contract_terms(contract_id, debtor_household_id, creditor_household_id, resource_code, quantity_milli)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'provisions', 1000)
	`, contractID, fixture.debtorID, fixture.creditorID); err != nil {
		t.Fatal(err)
	}
	arrivals := map[int64]int64{2: 2, 3: 3, 4: 5, 5: 7}
	for due := int64(2); due <= 6; due++ {
		status := "pending"
		var shipmentID *string
		if arrival, ok := arrivals[due]; ok {
			var id string
			if err := tx.QueryRow(ctx, `
				INSERT INTO shipments(
					world_id, sender_household_id, receiver_household_id,
					origin_location_id, destination_location_id, resource_code,
					quantity_milli, departure_tick, expected_arrival_tick, departure_game_day, expected_arrival_game_day, status
				) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, 'provisions', 1000, 0, $6::bigint, 0, ($6::bigint * 91) / 12, 'in_transit')
				RETURNING id::text
			`, fixture.worldID, fixture.debtorID, fixture.creditorID, originID, destinationID, arrival).Scan(&id); err != nil {
				t.Fatal(err)
			}
			shipmentID = &id
			status = "dispatched"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO contract_obligations(
				contract_id, debtor_household_id, creditor_household_id,
				resource_code, quantity_milli, due_arrival_tick, due_game_day, shipment_id, status
			) VALUES ($1::uuid, $2::uuid, $3::uuid, 'provisions', 1000, $4::bigint, ($4::bigint * 91) / 12, $5::uuid, $6::text)
		`, contractID, fixture.debtorID, fixture.creditorID, due, shipmentID, status); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func removeTrustTickFixture(t *testing.T, ctx context.Context, store *postgres.Store, worldID string) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM chronicle_entries WHERE household_id IN (SELECT id FROM households WHERE world_id = $1::uuid)`,
		`DELETE FROM relationship_events WHERE world_id = $1::uuid`,
		`DELETE FROM relationships WHERE world_id = $1::uuid`,
		`DELETE FROM contract_obligations WHERE contract_id IN (SELECT id FROM contracts WHERE world_id = $1::uuid)`,
		`DELETE FROM contracts WHERE world_id = $1::uuid`,
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

func createTickShipmentFixture(t *testing.T, ctx context.Context, store *postgres.Store) (string, string, string, string) {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var worldID string
	if err := tx.QueryRow(ctx, `
        INSERT INTO worlds(name, historical_start_date, current_tick, current_game_day, calendar_remainder, tick_duration_seconds, next_tick_at)
        VALUES ('tick shipment integration test', DATE '0980-01-01', 0, 0, 0, 172800, now() - interval '1 day')
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
				quantity_milli, departure_tick, expected_arrival_tick, departure_game_day, expected_arrival_game_day, status
			) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'provisions',30000,0,2,0,15,'in_transit')
        RETURNING id::text
	`, worldID, senderID, receiverID, originID, destinationID).Scan(&shipmentID); err != nil {
		t.Fatal(err)
	}
	var delayedShipmentID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO shipments(
			world_id, sender_household_id, receiver_household_id,
			origin_location_id, destination_location_id, resource_code,
			quantity_milli, departure_tick, expected_arrival_tick, departure_game_day, expected_arrival_game_day, status
		) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,'trade_goods',1000,0,6,0,45,'in_transit')
		RETURNING id::text
	`, worldID, senderID, receiverID, originID, destinationID).Scan(&delayedShipmentID); err != nil {
		t.Fatal(err)
	}
	var contractID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO contracts(
			world_id, party_a_household_id, party_b_household_id,
			starts_tick, ends_tick, interval_ticks, start_game_day, end_game_day, interval_days, game_day_schedule, status
		) VALUES ($1::uuid, $2::uuid, $3::uuid, 2, 2, 1, 15, 15, 1, false, 'active')
		RETURNING id::text
	`, worldID, senderID, receiverID).Scan(&contractID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO contract_terms(contract_id, debtor_household_id, creditor_household_id, resource_code, quantity_milli)
		VALUES
			($1::uuid, $2::uuid, $3::uuid, 'provisions', 30000),
			($1::uuid, $2::uuid, $3::uuid, 'wood', 1000),
			($1::uuid, $2::uuid, $3::uuid, 'trade_goods', 1000)
	`, contractID, senderID, receiverID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO contract_obligations(
			contract_id, debtor_household_id, creditor_household_id,
			resource_code, quantity_milli, due_arrival_tick, due_game_day, shipment_id, status
		) VALUES
			($1::uuid, $2::uuid, $3::uuid, 'provisions', 30000, 2, 15, $4::uuid, 'dispatched'),
			($1::uuid, $2::uuid, $3::uuid, 'wood', 1000, 2, 15, NULL, 'pending'),
			($1::uuid, $2::uuid, $3::uuid, 'trade_goods', 1000, 2, 15, $5::uuid, 'dispatched')
	`, contractID, senderID, receiverID, shipmentID, delayedShipmentID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return worldID, receiverID, shipmentID, contractID
}

func assertTickContract(
	t *testing.T,
	ctx context.Context,
	store *postgres.Store,
	contractID, wantLinkedStatus, wantUnlinkedStatus, wantDelayedStatus string,
	wantDelayedFulfilled *int64,
	wantRelationshipEvents int,
	wantTrust int,
) {
	t.Helper()
	wantContractStatus := "active"
	if wantUnlinkedStatus == "broken" {
		wantContractStatus = "broken"
	}
	var contractStatus string
	if err := store.Pool.QueryRow(ctx, `SELECT status FROM contracts WHERE id = $1::uuid`, contractID).Scan(&contractStatus); err != nil {
		t.Fatal(err)
	}
	if contractStatus != wantContractStatus {
		t.Fatalf("contract status = %s, want %s", contractStatus, wantContractStatus)
	}
	rows, err := store.Pool.Query(ctx, `
		SELECT resource_code, status, fulfilled_tick
		FROM contract_obligations
		WHERE contract_id = $1::uuid
		ORDER BY resource_code
	`, contractID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type state struct {
		status    string
		fulfilled *int64
	}
	states := make(map[string]state)
	for rows.Next() {
		var resource string
		var value state
		if err := rows.Scan(&resource, &value.status, &value.fulfilled); err != nil {
			t.Fatal(err)
		}
		states[resource] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	provisions := states["provisions"]
	if provisions.status != wantLinkedStatus {
		t.Fatalf("linked obligation = %+v, want %s", provisions, wantLinkedStatus)
	}
	if provisions.status == "fulfilled" && (provisions.fulfilled == nil || *provisions.fulfilled != 2) {
		t.Fatalf("linked obligation fulfillment = %+v, want tick 2", provisions)
	}
	if wood := states["wood"]; wood.status != wantUnlinkedStatus || wood.fulfilled != nil {
		t.Fatalf("unlinked obligation = %+v, want %s without fulfillment", wood, wantUnlinkedStatus)
	}
	delayed := states["trade_goods"]
	if delayed.status != wantDelayedStatus {
		t.Fatalf("delayed obligation = %+v, want %s", delayed, wantDelayedStatus)
	}
	if wantDelayedFulfilled == nil && delayed.fulfilled != nil {
		t.Fatalf("delayed obligation fulfillment = %d, want NULL", *delayed.fulfilled)
	}
	if wantDelayedFulfilled != nil && (delayed.fulfilled == nil || *delayed.fulfilled != *wantDelayedFulfilled) {
		t.Fatalf("delayed obligation fulfillment = %v, want %d", delayed.fulfilled, *wantDelayedFulfilled)
	}
	var eventCount int
	if err := store.Pool.QueryRow(ctx, `
		SELECT count(*) FROM relationship_events WHERE related_contract_id = $1::uuid
	`, contractID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != wantRelationshipEvents {
		t.Fatalf("relationship events = %d, want %d", eventCount, wantRelationshipEvents)
	}
	var relationshipCount int
	if err := store.Pool.QueryRow(ctx, `
		SELECT count(*)
		FROM relationships r
		JOIN contracts c
		  ON c.id = $1::uuid
		 AND r.source_household_id = c.party_b_household_id
		 AND r.target_household_id = c.party_a_household_id
	`, contractID).Scan(&relationshipCount); err != nil {
		t.Fatal(err)
	}
	wantRelationships := 0
	if wantRelationshipEvents > 0 {
		wantRelationships = 1
	}
	if relationshipCount != wantRelationships {
		t.Fatalf("relationship projections = %d, want %d", relationshipCount, wantRelationships)
	}
	if relationshipCount == 1 {
		var trust int
		var firstTick int64
		if err := store.Pool.QueryRow(ctx, `
			SELECT r.trust, r.first_interaction_tick
			FROM relationships r
			JOIN contracts c
			  ON c.id = $1::uuid
			 AND r.source_household_id = c.party_b_household_id
			 AND r.target_household_id = c.party_a_household_id
		`, contractID).Scan(&trust, &firstTick); err != nil {
			t.Fatal(err)
		}
		if trust != wantTrust || firstTick != 2 {
			t.Fatalf("relationship trust/first tick = %d/%d, want %d/2", trust, firstTick, wantTrust)
		}
	} else if wantTrust != 0 {
		t.Fatalf("relationship count = %d with expected trust %d", relationshipCount, wantTrust)
	}
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
		`DELETE FROM relationship_events WHERE world_id = $1::uuid`,
		`DELETE FROM relationships WHERE world_id = $1::uuid`,
		`DELETE FROM contract_obligations WHERE contract_id IN (SELECT id FROM contracts WHERE world_id = $1::uuid)`,
		`DELETE FROM contracts WHERE world_id = $1::uuid`,
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
