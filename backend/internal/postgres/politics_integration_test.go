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
	t.Cleanup(store.Close)
	world, household, actor, decision := createPoliticsFixture(t, ctx, store)
	t.Cleanup(func() { cleanupPoliticsFixture(t, ctx, store, world) })
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

func TestPoliticalSnapshotLevyRefusalAndInsufficientFunds(t *testing.T) {
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
	world, household, actor, _ := createPoliticsFixture(t, ctx, store)
	t.Cleanup(func() { cleanupPoliticsFixture(t, ctx, store, world) })
	service := application.NewPoliticsService(store, store)
	custom := createAdditionalPoliticalDecision(t, ctx, store, world, household, actor, "political_levy", map[string]any{"wood_cost_milli": 13000, "silver_cost_milli": 4000, "honor_standing_delta": 7, "refuse_standing_delta": -3})
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: custom, HouseholdID: household, Option: "pay_wood"}); err != nil {
		t.Fatal(err)
	}
	var wood int64
	var score int
	if err := store.Pool.QueryRow(ctx, `SELECT quantity_milli FROM resource_stocks WHERE household_id=$1::uuid AND resource_code='wood'`, household).Scan(&wood); err != nil {
		t.Fatal(err)
	}
	if wood != 7000 {
		t.Fatalf("snapshot wood=%d, want 7000", wood)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 7 {
		t.Fatalf("snapshot score=%d, want 7", score)
	}
	refusal := createAdditionalPoliticalDecision(t, ctx, store, world, household, actor, "political_levy", map[string]any{"wood_cost_milli": 18000, "silver_cost_milli": 6000, "honor_standing_delta": 10, "refuse_standing_delta": -5})
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: refusal, HouseholdID: household, Option: "refuse"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 2 {
		t.Fatalf("refusal score=%d, want 2", score)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE resource_stocks SET quantity_milli=10000 WHERE household_id=$1::uuid AND resource_code='wood'`, household); err != nil {
		t.Fatal(err)
	}
	insufficient := createAdditionalPoliticalDecision(t, ctx, store, world, household, actor, "political_levy", map[string]any{"wood_cost_milli": 18000, "silver_cost_milli": 6000, "honor_standing_delta": 10, "refuse_standing_delta": -5})
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: insufficient, HouseholdID: household, Option: "pay_wood"}); err == nil {
		t.Fatal("insufficient levy should fail")
	}
	var status string
	var chronicle int
	if err := store.Pool.QueryRow(ctx, `SELECT status FROM household_decisions WHERE id=$1::uuid`, insufficient).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("insufficient status=%s", status)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chronicle_entries WHERE related_household_decision_id=$1::uuid AND entry_type='political_demand_resolved'`, insufficient).Scan(&chronicle); err != nil {
		t.Fatal(err)
	}
	if chronicle != 0 {
		t.Fatalf("insufficient chronicles=%d", chronicle)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT quantity_milli FROM resource_stocks WHERE household_id=$1::uuid AND resource_code='wood'`, household).Scan(&wood); err != nil {
		t.Fatal(err)
	}
	if wood != 10_000 {
		t.Fatalf("insufficient levy changed wood to %d", wood)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 2 {
		t.Fatalf("insufficient levy changed score to %d", score)
	}
}

func TestPoliticalSilverLevyAndScoreClamping(t *testing.T) {
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
	world, household, actor, silverDecision := createPoliticsFixture(t, ctx, store)
	t.Cleanup(func() { cleanupPoliticsFixture(t, ctx, store, world) })
	service := application.NewPoliticsService(store, store)
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: silverDecision, HouseholdID: household, Option: "pay_silver"}); err != nil {
		t.Fatal(err)
	}
	var silver int64
	var score int
	if err := store.Pool.QueryRow(ctx, `SELECT quantity_milli FROM resource_stocks WHERE household_id=$1::uuid AND resource_code='silver'`, household).Scan(&silver); err != nil {
		t.Fatal(err)
	}
	if silver != 24_000 {
		t.Fatalf("silver=%d, want 24000", silver)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("silver levy score=%d, want 10", score)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE political_relationships SET standing=95 WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor); err != nil {
		t.Fatal(err)
	}
	upper := createAdditionalPoliticalDecision(t, ctx, store, world, household, actor, "political_levy", defaultLevyTerms())
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: upper, HouseholdID: household, Option: "pay_wood"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("upper clamp=%d, want 100", score)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE political_relationships SET standing=-98 WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor); err != nil {
		t.Fatal(err)
	}
	lower := createAdditionalPoliticalDecision(t, ctx, store, world, household, actor, "political_levy", defaultLevyTerms())
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: lower, HouseholdID: household, Option: "refuse"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != -100 {
		t.Fatalf("lower clamp=%d, want -100", score)
	}
}

func TestPoliticalLaborServiceAndAssignmentConflict(t *testing.T) {
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
	world, household, actor, _ := createPoliticsFixture(t, ctx, store)
	t.Cleanup(func() { cleanupPoliticsFixture(t, ctx, store, world) })
	var worker string
	if err := store.Pool.QueryRow(ctx, `SELECT id::text FROM characters WHERE household_id=$1::uuid`, household).Scan(&worker); err != nil {
		t.Fatal(err)
	}
	service := application.NewPoliticsService(store, store)
	demand := createAdditionalPoliticalDecision(t, ctx, store, world, household, actor, "political_labor_service", map[string]any{"service_ticks": 4, "honor_standing_delta": 10, "refuse_standing_delta": -5})
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: demand, HouseholdID: household, Option: "serve", CharacterID: worker}); err != nil {
		t.Fatal(err)
	}
	var activity, intensity string
	var start, end int64
	if err := store.Pool.QueryRow(ctx, `SELECT activity_type,intensity,starts_tick,ends_tick FROM assignments WHERE household_id=$1::uuid AND activity_type='ruler_service'`, household).Scan(&activity, &intensity, &start, &end); err != nil {
		t.Fatal(err)
	}
	if activity != "ruler_service" || intensity != "normal" || start != 5 || end != 8 {
		t.Fatalf("service assignment=%s/%s/%d/%d", activity, intensity, start, end)
	}
	var score, chronicles int
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("service score=%d, want 10", score)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chronicle_entries WHERE related_household_decision_id=$1::uuid AND entry_type='political_demand_resolved'`, demand).Scan(&chronicles); err != nil {
		t.Fatal(err)
	}
	if chronicles != 1 {
		t.Fatalf("service chronicles=%d, want 1", chronicles)
	}
	conflict := createAdditionalPoliticalDecision(t, ctx, store, world, household, actor, "political_labor_service", map[string]any{"service_ticks": 4, "honor_standing_delta": 10, "refuse_standing_delta": -5})
	var conflictWorker string
	if err := store.Pool.QueryRow(ctx, `INSERT INTO characters(household_id,name,birth_date,labor_capacity_milli) VALUES ($1::uuid,'Conflicted worker',DATE '0960-01-01',1000) RETURNING id::text`, household).Scan(&conflictWorker); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `INSERT INTO assignments(household_id,character_id,activity_type,intensity,starts_tick,ends_tick,status) VALUES ($1::uuid,$2::uuid,'fishing','normal',5,8,'planned')`, household, conflictWorker); err != nil {
		t.Fatal(err)
	}
	projection, err := store.GetHouseholdPolitics(ctx, household)
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range projection.Decisions {
		if decision.ID != conflict {
			continue
		}
		for _, candidate := range decision.EligibleCharacters {
			if candidate.ID == conflictWorker {
				t.Fatal("overlapping worker must not be advertised as eligible")
			}
		}
	}
	if err := service.Respond(ctx, application.RespondPoliticalDemandCommand{DecisionID: conflict, HouseholdID: household, Option: "serve", CharacterID: conflictWorker}); err == nil {
		t.Fatal("overlapping service should fail")
	}
	var status string
	if err := store.Pool.QueryRow(ctx, `SELECT status FROM household_decisions WHERE id=$1::uuid`, conflict).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("conflict status=%s", status)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM assignments WHERE character_id=$1::uuid AND activity_type='ruler_service'`, conflictWorker).Scan(&chronicles); err != nil {
		t.Fatal(err)
	}
	if chronicles != 0 {
		t.Fatalf("conflict created %d ruler-service assignments", chronicles)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("conflict changed score to %d", score)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chronicle_entries WHERE related_household_decision_id=$1::uuid AND entry_type='political_demand_resolved'`, conflict).Scan(&chronicles); err != nil {
		t.Fatal(err)
	}
	if chronicles != 0 {
		t.Fatalf("conflict created %d chronicles", chronicles)
	}
}

func TestPoliticalExpiryUsesSnapshottedRefusalExactlyOnce(t *testing.T) {
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
	world, household, actor, _ := createPoliticsFixture(t, ctx, store)
	t.Cleanup(func() { cleanupPoliticsFixture(t, ctx, store, world) })
	var location string
	if err := store.Pool.QueryRow(ctx, `SELECT location_id::text FROM households WHERE id=$1::uuid`, household).Scan(&location); err != nil {
		t.Fatal(err)
	}
	var event, decision string
	terms := []byte(`{"wood_cost_milli":18000,"silver_cost_milli":6000,"honor_standing_delta":10,"refuse_standing_delta":-3}`)
	if err := store.Pool.QueryRow(ctx, `INSERT INTO world_events(world_id,location_id,political_actor_id,event_type,starts_tick,ends_tick) VALUES ($1::uuid,$2::uuid,$3::uuid,'political_levy',0,1) RETURNING id::text`, world, location, actor).Scan(&event); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO household_decisions(household_id,world_id,world_event_id,decision_type,available_from_tick,expires_tick,available_from_game_day,expires_game_day,default_option,parameters) VALUES ($1::uuid,$2::uuid,$3::uuid,'political_levy',0,1,0,7,'refuse',$4::jsonb) RETURNING id::text`, household, world, event, terms).Scan(&decision); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE worlds SET next_tick_at = now() - interval '1 hour' WHERE id=$1::uuid`, world); err != nil {
		t.Fatal(err)
	}
	processor := application.NewTickProcessor(store)
	processed, err := processor.ProcessOneDueWorld(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected due fixture world to process")
	}
	var status, option string
	var delta int
	if err := store.Pool.QueryRow(ctx, `SELECT status,selected_option,standing_delta FROM household_decisions WHERE id=$1::uuid`, decision).Scan(&status, &option, &delta); err != nil {
		t.Fatal(err)
	}
	if status != "auto_resolved" || option != "refuse" || delta != -3 {
		t.Fatalf("expiry=%s/%s/%d", status, option, delta)
	}
	var score, events int
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != -3 {
		t.Fatalf("expiry score=%d", score)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chronicle_entries WHERE related_household_decision_id=$1::uuid AND entry_type='political_demand_auto_resolved'`, decision).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("expiry chronicle=%d", events)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT standing FROM political_relationships WHERE household_id=$1::uuid AND political_actor_id=$2::uuid`, household, actor).Scan(&score); err != nil {
		t.Fatal(err)
	}
	if score != -3 {
		t.Fatalf("expiry retry changed score to %d", score)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chronicle_entries WHERE related_household_decision_id=$1::uuid AND entry_type='political_demand_auto_resolved'`, decision).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("expiry retry chronicles=%d", events)
	}
}

func defaultLevyTerms() map[string]any {
	return map[string]any{"wood_cost_milli": 18000, "silver_cost_milli": 6000, "honor_standing_delta": 10, "refuse_standing_delta": -5}
}

func cleanupPoliticsFixture(t *testing.T, ctx context.Context, store *Store, world string) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM chronicle_entries WHERE household_id IN (SELECT id FROM households WHERE world_id=$1::uuid) OR related_political_actor_id IN (SELECT id FROM political_actors WHERE world_id=$1::uuid)`,
		`DELETE FROM political_relationships WHERE world_id=$1::uuid`,
		`DELETE FROM household_decisions WHERE world_id=$1::uuid`,
		`DELETE FROM world_events WHERE world_id=$1::uuid`,
		`DELETE FROM political_actors WHERE world_id=$1::uuid`,
		`DELETE FROM worlds WHERE id=$1::uuid`,
	} {
		if _, err := store.Pool.Exec(ctx, statement, world); err != nil {
			t.Errorf("cleanup politics fixture: %v", err)
			return
		}
	}
}

func createAdditionalPoliticalDecision(t *testing.T, ctx context.Context, store *Store, world, household, actor, demand string, terms map[string]any) string {
	t.Helper()
	var location string
	if err := store.Pool.QueryRow(ctx, `SELECT location_id::text FROM households WHERE id=$1::uuid`, household).Scan(&location); err != nil {
		t.Fatal(err)
	}
	var event, decision string
	if err := store.Pool.QueryRow(ctx, `INSERT INTO world_events(world_id,location_id,political_actor_id,event_type,starts_tick,ends_tick) VALUES ($1::uuid,$2::uuid,$3::uuid,$4,1,5) RETURNING id::text`, world, location, actor, demand).Scan(&event); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(terms)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO household_decisions(household_id,world_id,world_event_id,decision_type,available_from_tick,expires_tick,available_from_game_day,expires_game_day,default_option,parameters) VALUES ($1::uuid,$2::uuid,$3::uuid,$4,1,5,7,37,'refuse',$5::jsonb) RETURNING id::text`, household, world, event, demand, data).Scan(&decision); err != nil {
		t.Fatal(err)
	}
	return decision
}

func createPoliticsFixture(t *testing.T, ctx context.Context, store *Store) (world, household, actor, decision string) {
	t.Helper()
	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `INSERT INTO worlds(name,historical_start_date,current_game_day,calendar_remainder,tick_duration_seconds,next_tick_at) VALUES ('politics integration',DATE '0980-01-01',0,0,3600,now() + interval '100 years') RETURNING id::text`).Scan(&world); err != nil {
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
	if err := tx.QueryRow(ctx, `INSERT INTO household_decisions(household_id,world_id,world_event_id,decision_type,available_from_tick,expires_tick,available_from_game_day,expires_game_day,default_option,parameters) VALUES ($1::uuid,$2::uuid,$3::uuid,'political_levy',1,5,7,37,'refuse',$4::jsonb) RETURNING id::text`, household, world, event, terms).Scan(&decision); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return
}
