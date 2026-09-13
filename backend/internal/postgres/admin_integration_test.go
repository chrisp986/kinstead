//go:build postgres

package postgres

import (
	"context"
	"os"
	"testing"

	"game/backend/internal/port"
)

func TestAdminProjectionsUseSafeBoundedQueries(t *testing.T) {
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
	t.Cleanup(func() { removeChronicleFixture(t, ctx, store, worldID, locationID, householdID, characterID) })
	var playerID string
	if err := store.Pool.QueryRow(ctx, `INSERT INTO players(external_auth_subject) VALUES($1) RETURNING id::text`, "admin-projection-"+householdID).Scan(&playerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = store.Pool.Exec(ctx, `DELETE FROM players WHERE id=$1::uuid`, playerID) })
	if _, err := store.GrantAdminMembership(ctx, playerID); err != nil {
		t.Fatal(err)
	}

	worlds, err := store.ListAdminWorlds(ctx, port.AdminCursor{}, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, world := range worlds {
		if world.ID == worldID {
			found = true
			if world.Heartbeats == nil || world.RecentFailures == nil {
				t.Fatalf("world collection fields must be arrays: %+v", world)
			}
		}
	}
	if !found {
		t.Fatalf("world %s not found in bounded admin list", worldID)
	}

	household, err := store.GetAdminHousehold(ctx, householdID)
	if err != nil {
		t.Fatal(err)
	}
	if household.ID != householdID || household.OwnerPlayerID != nil || household.Characters == nil || household.IncomingShipments == nil || household.OutgoingShipments == nil {
		t.Fatalf("unexpected household projection: %+v", household)
	}
	if household.GameMoment != nil {
		t.Fatal("legacy household must not fabricate an hourly game moment")
	}

	sessions, err := store.ListAdminSessions(ctx, playerID, port.AdminCursor{}, 50)
	if err != nil {
		t.Fatal(err)
	}
	if sessions == nil {
		t.Fatal("sessions must be a non-nil array")
	}
}
