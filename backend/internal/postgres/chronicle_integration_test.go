//go:build postgres

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"game/backend/internal/simulation"
)

func TestAssignmentChronicleLifecycle(t *testing.T) {
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

	worldID, locationID, householdID, characterID := createChronicleFixture(t, ctx, store)
	t.Cleanup(func() {
		removeChronicleFixture(t, ctx, store, worldID, locationID, householdID, characterID)
	})

	assignment, err := store.CreateAssignment(ctx, householdID, characterID, "fishing", "normal", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := store.ListHouseholdChronicle(ctx, householdID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].EntryType != "assignment_scheduled" {
		t.Fatalf("scheduled entries = %+v", entries)
	}
	if entries[0].RelatedAssignmentID == nil || *entries[0].RelatedAssignmentID != assignment.ID ||
		entries[0].SubjectCharacterName == nil || *entries[0].SubjectCharacterName != "Chronicle worker" {
		t.Fatalf("scheduled entry references = %+v", entries[0])
	}

	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result := simulation.TickResult{State: simulation.HouseholdState{
		Tick: 1,
		Characters: []simulation.Character{{
			ID: characterID, Name: "Chronicle worker", LaborPermille: 1000,
		}},
	}}
	if err := store.SaveHouseholdTick(ctx, tx, householdID, result); err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	entries, err = store.ListHouseholdChronicle(ctx, householdID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].EntryType != "assignment_completed" || entries[0].OccurredTick != 1 {
		t.Fatalf("completed entries = %+v", entries)
	}

	_, err = store.ListHouseholdChronicle(ctx, "00000000-0000-0000-0000-000000000099")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing household error = %v, want pgx.ErrNoRows", err)
	}
}

func createChronicleFixture(t *testing.T, ctx context.Context, store *Store) (string, string, string, string) {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var worldID, locationID, householdID, characterID string
	if err := tx.QueryRow(ctx, `
        INSERT INTO worlds(name, historical_start_date, current_tick, tick_duration_seconds, next_tick_at)
        VALUES ('chronicle integration test', DATE '0980-01-01', 0, 3600, now() + interval '1 day')
        RETURNING id::text
    `).Scan(&worldID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO locations(world_id, name, location_type)
        VALUES ($1::uuid, 'Chronicle place', 'farm') RETURNING id::text
    `, worldID).Scan(&locationID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
        INSERT INTO households(world_id, location_id, name, created_tick)
        VALUES ($1::uuid, $2::uuid, 'Chronicle household', 0) RETURNING id::text
    `, worldID, locationID).Scan(&householdID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO characters(household_id, name, birth_date, labor_capacity_milli)
		VALUES ($1::uuid, 'Chronicle worker', DATE '0970-01-01', 1000) RETURNING id::text
    `, householdID).Scan(&characterID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return worldID, locationID, householdID, characterID
}

func removeChronicleFixture(t *testing.T, ctx context.Context, store *Store, worldID, locationID, householdID, characterID string) {
	t.Helper()
	for _, cleanup := range []struct {
		statement string
		id        string
	}{
		{`DELETE FROM chronicle_entries WHERE household_id = $1::uuid`, householdID},
		{`DELETE FROM assignments WHERE household_id = $1::uuid`, householdID},
		{`DELETE FROM resource_stocks WHERE household_id = $1::uuid`, householdID},
		{`DELETE FROM characters WHERE id = $1::uuid`, characterID},
		{`DELETE FROM households WHERE id = $1::uuid`, householdID},
		{`DELETE FROM locations WHERE id = $1::uuid`, locationID},
		{`DELETE FROM worlds WHERE id = $1::uuid`, worldID},
	} {
		if _, err := store.Pool.Exec(ctx, cleanup.statement, cleanup.id); err != nil {
			t.Errorf("clean chronicle fixture: %v", err)
		}
	}
}
