//go:build postgres

package postgres

import (
	"context"
	"os"
	"testing"

	"game/backend/internal/application"
)

func TestReturnHistoryPrioritizesOldShortageAndRequiresAcknowledgement(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	worldID, locationID, householdID, characterID := createChronicleFixture(t, ctx, store)
	t.Cleanup(func() { removeChronicleFixture(t, ctx, store, worldID, locationID, householdID, characterID) })
	_, err = store.Pool.Exec(ctx, `UPDATE worlds SET current_tick=200,current_game_day=200,game_days_per_tick_num=1,game_days_per_tick_den=1 WHERE id=$1::uuid`, worldID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO chronicle_entries(household_id,occurred_tick,occurred_game_day,entry_type,data)
	 SELECT $1::uuid,n,n,'assignment_completed','{}'::jsonb FROM generate_series(2,150) n`, householdID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Pool.Exec(ctx, `INSERT INTO chronicle_entries(household_id,occurred_tick,occurred_game_day,entry_type,data)
	 VALUES($1::uuid,1,1,'food_shortage','{"food_shortage_milli":4900}')`, householdID)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewReportService(store)
	for i := 0; i < 2; i++ {
		report, err := service.FarmReport(ctx, householdID)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.SinceYouWereAway) != 1 || report.SinceYouWereAway[0].EntryType != "food_shortage" {
			t.Fatalf("return history lost important older event: %+v", report.SinceYouWereAway)
		}
	}
	if err := service.Acknowledge(ctx, householdID, 201); err == nil {
		t.Fatal("accepted future acknowledgement")
	}
	if err := service.Acknowledge(ctx, householdID, 200); err != nil {
		t.Fatal(err)
	}
	report, err := service.FarmReport(ctx, householdID)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.SinceYouWereAway) != 0 {
		t.Fatalf("acknowledged history returned: %+v", report.SinceYouWereAway)
	}
}
