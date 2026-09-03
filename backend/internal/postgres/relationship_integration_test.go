//go:build postgres

package postgres

import (
	"context"
	"os"
	"testing"

	contractdomain "game/backend/internal/domain/contract"
	relationshipdomain "game/backend/internal/domain/relationship"
	shipmentdomain "game/backend/internal/domain/shipment"
)

type relationshipTrustFixture struct {
	worldID    string
	debtorID   string
	creditorID string
	contractID string
	obligation []string
	shipment   []string
}

func TestRelationshipEventTrustApplicationIsExactlyOnceAndClamped(t *testing.T) {
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
	fixture := createRelationshipTrustFixture(t, ctx, store)
	t.Cleanup(func() { removeRelationshipTrustFixture(t, ctx, store, fixture.worldID) })

	event := relationshipdomain.Event{
		WorldID: contractdomain.WorldID(fixture.worldID), SourceHouseholdID: contractdomain.HouseholdID(fixture.creditorID), TargetHouseholdID: contractdomain.HouseholdID(fixture.debtorID),
		Type: relationshipdomain.EventContractFulfilled, TrustDelta: relationshipdomain.TrustDeltaContractFulfilled,
		OccurredTick: 10, ContractID: contractdomain.ID(fixture.contractID), ObligationID: contractdomain.ObligationID(fixture.obligation[0]), ShipmentID: shipmentdomain.ID(fixture.shipment[0]),
		ResourceType: "provisions", QuantityMilli: 1_000, DueArrivalTick: 10, ActualFulfillmentTick: contractTickPtr(10),
	}
	persistEvent(t, ctx, store, event)
	// A retry in a new transaction must hit the obligation-outcome unique index
	// and skip the trust update as well as the duplicate event.
	persistEvent(t, ctx, store, event)
	assertRelationshipTrust(t, ctx, store, fixture, 2, 1)
	conflictingOutcome := event
	conflictingOutcome.Type = relationshipdomain.EventContractBroken
	conflictingOutcome.TrustDelta = relationshipdomain.TrustDeltaContractBroken
	conflictingOutcome.ShipmentID = ""
	conflictingOutcome.OccurredTick = 13
	conflictingOutcome.ActualFulfillmentTick = nil
	persistEvent(t, ctx, store, conflictingOutcome)
	assertRelationshipTrust(t, ctx, store, fixture, 2, 1)
	var obligationEvents int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM relationship_events WHERE related_obligation_id = $1::uuid`, fixture.obligation[0]).Scan(&obligationEvents); err != nil {
		t.Fatal(err)
	}
	if obligationEvents != 1 {
		t.Fatalf("events for obligation = %d, want 1", obligationEvents)
	}

	if _, err := store.Pool.Exec(ctx, `UPDATE relationships SET trust = 99 WHERE source_household_id = $1::uuid AND target_household_id = $2::uuid`, fixture.creditorID, fixture.debtorID); err != nil {
		t.Fatal(err)
	}
	event.ObligationID = contractdomain.ObligationID(fixture.obligation[1])
	event.ShipmentID = shipmentdomain.ID(fixture.shipment[1])
	event.OccurredTick = 11
	event.DueArrivalTick = 11
	event.ActualFulfillmentTick = contractTickPtr(11)
	persistEvent(t, ctx, store, event)
	assertRelationshipTrust(t, ctx, store, fixture, 100, 2)

	if _, err := store.Pool.Exec(ctx, `UPDATE relationships SET trust = -97 WHERE source_household_id = $1::uuid AND target_household_id = $2::uuid`, fixture.creditorID, fixture.debtorID); err != nil {
		t.Fatal(err)
	}
	event.Type = relationshipdomain.EventContractBroken
	event.TrustDelta = relationshipdomain.TrustDeltaContractBroken
	event.ObligationID = contractdomain.ObligationID(fixture.obligation[2])
	event.ShipmentID = ""
	event.OccurredTick = 15
	event.DueArrivalTick = 12
	event.ActualFulfillmentTick = nil
	persistEvent(t, ctx, store, event)
	assertRelationshipTrust(t, ctx, store, fixture, -100, 3)

	var reverseCount int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM relationships WHERE source_household_id = $1::uuid AND target_household_id = $2::uuid`, fixture.debtorID, fixture.creditorID).Scan(&reverseCount); err != nil {
		t.Fatal(err)
	}
	if reverseCount != 0 {
		t.Fatalf("reverse relationship count = %d, want 0", reverseCount)
	}
}

func persistEvent(t *testing.T, ctx context.Context, store *Store, event relationshipdomain.Event) {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistRelationshipEvent(ctx, tx, event); err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func assertRelationshipTrust(t *testing.T, ctx context.Context, store *Store, fixture relationshipTrustFixture, wantTrust, wantEvents int) {
	t.Helper()
	var trust, events int
	if err := store.Pool.QueryRow(ctx, `SELECT trust FROM relationships WHERE source_household_id = $1::uuid AND target_household_id = $2::uuid`, fixture.creditorID, fixture.debtorID).Scan(&trust); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM relationship_events WHERE world_id = $1::uuid`, fixture.worldID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if trust != wantTrust || events != wantEvents {
		t.Fatalf("relationship trust/events = %d/%d, want %d/%d", trust, events, wantTrust, wantEvents)
	}
}

func createRelationshipTrustFixture(t *testing.T, ctx context.Context, store *Store) relationshipTrustFixture {
	t.Helper()
	var fixture relationshipTrustFixture
	if err := store.Pool.QueryRow(ctx, `
		INSERT INTO worlds(name, historical_start_date, current_tick, tick_duration_seconds, next_tick_at)
		VALUES ('relationship trust integration test', DATE '0980-01-01', 0, 3600, now() + interval '1 day')
		RETURNING id::text
	`).Scan(&fixture.worldID); err != nil {
		t.Fatal(err)
	}
	var originID, destinationID string
	if err := store.Pool.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'relationship origin', 'farm') RETURNING id::text`, fixture.worldID).Scan(&originID); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'relationship destination', 'farm') RETURNING id::text`, fixture.worldID).Scan(&destinationID); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'relationship debtor', 0) RETURNING id::text`, fixture.worldID, originID).Scan(&fixture.debtorID); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'relationship creditor', 0) RETURNING id::text`, fixture.worldID, destinationID).Scan(&fixture.creditorID); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `
		INSERT INTO contracts(world_id, party_a_household_id, party_b_household_id, starts_tick, ends_tick, interval_ticks, status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 10, 12, 1, 'active')
		RETURNING id::text
	`, fixture.worldID, fixture.debtorID, fixture.creditorID).Scan(&fixture.contractID); err != nil {
		t.Fatal(err)
	}
	for i, value := range []struct {
		resource string
		due      int64
		arrival  *int64
	}{
		{resource: "provisions", due: 10, arrival: int64Ptr(10)},
		{resource: "wood", due: 11, arrival: int64Ptr(11)},
		{resource: "trade_goods", due: 12},
	} {
		var shipmentID *string
		status := "pending"
		if value.arrival != nil {
			var id string
			if err := store.Pool.QueryRow(ctx, `
				INSERT INTO shipments(
					world_id, sender_household_id, receiver_household_id,
					origin_location_id, destination_location_id, resource_code,
					quantity_milli, departure_tick, expected_arrival_tick, actual_arrival_tick, status
				) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, $6, 1000, 0, $7, $7, 'arrived')
				RETURNING id::text
			`, fixture.worldID, fixture.debtorID, fixture.creditorID, originID, destinationID, value.resource, *value.arrival).Scan(&id); err != nil {
				t.Fatal(err)
			}
			shipmentID = &id
			status = "fulfilled"
			fixture.shipment = append(fixture.shipment, id)
		} else {
			fixture.shipment = append(fixture.shipment, "")
		}
		var obligationID string
		if err := store.Pool.QueryRow(ctx, `
			INSERT INTO contract_obligations(
				contract_id, debtor_household_id, creditor_household_id,
				resource_code, quantity_milli, due_arrival_tick, shipment_id, status, fulfilled_tick
			) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 1000, $5, $6::uuid, $7, $8)
			RETURNING id::text
		`, fixture.contractID, fixture.debtorID, fixture.creditorID, value.resource, value.due, shipmentID, status, value.arrival).Scan(&obligationID); err != nil {
			t.Fatalf("obligation %d: %v", i, err)
		}
		fixture.obligation = append(fixture.obligation, obligationID)
	}
	return fixture
}

func removeRelationshipTrustFixture(t *testing.T, ctx context.Context, store *Store, worldID string) {
	t.Helper()
	for _, statement := range []string{
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

func contractTickPtr(value int64) *contractdomain.Tick {
	tick := contractdomain.Tick(value)
	return &tick
}

func int64Ptr(value int64) *int64 {
	return &value
}
