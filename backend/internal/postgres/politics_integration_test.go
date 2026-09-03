//go:build postgres

package postgres

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"game/backend/internal/application"
)

func TestPoliticalLevyResponseIsTransactionalAndIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	world, household, actor, decision := createPoliticsFixture(t, ctx, store)
	t.Cleanup(func() { _, _ = store.Pool.Exec(ctx, `DELETE FROM worlds WHERE id=$1::uuid`, world) })
	service := application.NewPoliticsService(store, store)
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: decision, HouseholdID: household, Option: "pay_wood"}); err != nil {
		t.Fatal(err)
	}
	var stock int64
	if err := store.Pool.QueryRow(ctx, `SELECT quantity_milli FROM resource_stocks WHERE household_id=$1::uuid AND resource_code='wood'`, household).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 2_000 {
		t.Fatalf("wood=%d, want 2000", stock)
	}
	var score int
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("score=%d, want 10", score)
	}
	var events int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chronicle_entries WHERE related_household_decision_id=$1::uuid AND entry_type='political_demand_resolved'`, decision).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("resolution chronicles=%d, want 1", events)
	}
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: decision, HouseholdID: household, Option: "pay_wood"}); err == nil {
		t.Fatal("retry should not resolve an already resolved demand")
	}
	if err := store.Pool.QueryRow(ctx, `SELECT quantity_milli FROM resource_stocks WHERE household_id=$1::uuid AND resource_code='wood'`, household).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 2_000 {
		t.Fatalf("retry changed wood to %d", stock)
	}
}

func createPoliticsFixture(t *testing.T, ctx context.Context, store *Store) (world, household, actor, decision string) {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `INSERT INTO worlds(name,historical_start_date,next_tick_at) VALUES ('politics integration',DATE '0980-01-01',now()) RETURNING id::text`).Scan(&world); err != nil {
		t.Fatal(err)
	}
	var location string
	if err := tx.QueryRow(ctx, `INSERT INTO locations(world_id,name,location_type) VALUES ($1::uuid,'Politics place','farm') RETURNING id::text`, world).Scan(&location); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO households(world_id,location_id,name,created_tick) VALUES ($1::uuid,$2::uuid,'Politics household',0) RETURNING id::text`, world, location).Scan(&household); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO characters(household_id,name,birth_date,labor_capacity_milli) VALUES ($1::uuid,'Politics worker',DATE '0960-01-01',1000)`, household); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO resource_stocks(household_id,resource_code,quantity_milli) VALUES ($1::uuid,'wood',20000),($1::uuid,'silver',30000),($1::uuid,'provisions',100000)`, household); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO political_actors(world_id,location_id,name,actor_type) VALUES ($1::uuid,$2::uuid,'Jarl Test','jarl') RETURNING id::text`, world, location).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	var event string
	if err := tx.QueryRow(ctx, `INSERT INTO world_events(world_id,location_id,political_actor_id,event_type,starts_tick,ends_tick) VALUES ($1::uuid,$2::uuid,$3::uuid,'political_levy',1,5) RETURNING id::text`, world, location, actor).Scan(&event); err != nil {
		t.Fatal(err)
	}
	terms, _ := json.Marshal(map[string]any{"wood_cost_milli": 18000, "silver_cost_milli": 6000, "honor_standing_delta": 10, "refuse_standing_delta": -5})
	if err := tx.QueryRow(ctx, `INSERT INTO household_decisions(household_id,world_id,world_event_id,decision_type,available_from_tick,expires_tick,default_option,parameters) VALUES ($1::uuid,$2::uuid,$3::uuid,'political_levy',1,5,'refuse',$4::jsonb) RETURNING id::text`, household, world, event, terms).Scan(&decision); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return
}
