//go:build postgres

package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
)

func TestCreateAssignmentOverlapRules(t *testing.T) {
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
	clear := func(t *testing.T) {
		t.Helper()
		if _, err := store.Pool.Exec(ctx, `DELETE FROM chronicle_entries WHERE household_id=$1::uuid`, householdID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Pool.Exec(ctx, `DELETE FROM assignments WHERE household_id=$1::uuid`, householdID); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("normal player work blocks player work", func(t *testing.T) {
		clear(t)
		if _, err := store.CreateAssignment(ctx, householdID, characterID, "fishing", "normal", 1, 3); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateAssignment(ctx, householdID, characterID, "agriculture", "normal", 2, 2); !errors.Is(err, ErrAssignmentConflict) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("future emergency may be replaced", func(t *testing.T) {
		clear(t)
		if _, err := store.Pool.Exec(ctx, `INSERT INTO assignments(household_id,character_id,activity_type,intensity,starts_tick,ends_tick,status,metadata) VALUES($1::uuid,$2::uuid,'fishing','normal',3,3,'planned','{"source":"emergency"}')`, householdID, characterID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateAssignment(ctx, householdID, characterID, "agriculture", "normal", 3, 3); err != nil {
			t.Fatal(err)
		}
		var status string
		if err := store.Pool.QueryRow(ctx, `SELECT status FROM assignments WHERE character_id=$1::uuid AND metadata->>'source'='emergency'`, characterID).Scan(&status); err != nil || status != "cancelled" {
			t.Fatalf("status=%q err=%v", status, err)
		}
	})
	t.Run("active emergency blocks", func(t *testing.T) {
		clear(t)
		if _, err := store.Pool.Exec(ctx, `INSERT INTO assignments(household_id,character_id,activity_type,intensity,starts_tick,ends_tick,status,metadata) VALUES($1::uuid,$2::uuid,'fishing','normal',0,2,'active','{"source":"emergency"}')`, householdID, characterID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateAssignment(ctx, householdID, characterID, "agriculture", "normal", 1, 1); !errors.Is(err, ErrAssignmentConflict) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("political service blocks", func(t *testing.T) {
		clear(t)
		if _, err := store.Pool.Exec(ctx, `INSERT INTO assignments(household_id,character_id,activity_type,intensity,starts_tick,ends_tick,status,metadata) VALUES($1::uuid,$2::uuid,'ruler_service','normal',1,4,'planned','{"source":"politics"}')`, householdID, characterID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateAssignment(ctx, householdID, characterID, "fishing", "normal", 1, 1); !errors.Is(err, ErrAssignmentConflict) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("concurrent attempts yield one conflict", func(t *testing.T) {
		clear(t)
		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for _, activity := range []string{"fishing", "agriculture"} {
			wg.Add(1)
			go func(activity string) {
				defer wg.Done()
				<-start
				_, err := store.CreateAssignment(ctx, householdID, characterID, activity, "normal", 1, 2)
				errs <- err
			}(activity)
		}
		close(start)
		wg.Wait()
		close(errs)
		successes, conflicts := 0, 0
		for err := range errs {
			if err == nil {
				successes++
			} else if errors.Is(err, ErrAssignmentConflict) {
				conflicts++
			} else {
				t.Fatalf("unexpected error %v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("success/conflict=%d/%d", successes, conflicts)
		}
	})
}
