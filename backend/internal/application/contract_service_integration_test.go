//go:build postgres

package application

import (
	"context"
	"os"
	"testing"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/postgres"
)

type contractFixture struct {
	worldID string
	partyA  string
	partyB  string
}

func TestContractProposalPersistenceAndRollback(t *testing.T) {
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
	fixture := createContractFixture(t, ctx, store)
	t.Cleanup(func() { removeContractFixture(t, ctx, store, fixture.worldID) })
	service := NewContractService(store)

	command := ProposeContractCommand{
		ProposerHouseholdID: fixture.partyA, CounterpartyHouseholdID: fixture.partyB,
		StartsTick: 7, EndsTick: 13, IntervalTicks: 3,
		Terms: []ContractTermIntent{{DebtorHouseholdID: fixture.partyA, CreditorHouseholdID: fixture.partyB, ResourceType: "provisions", QuantityMilli: 10_000}},
	}
	created, err := service.Propose(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.Get(ctx, string(created.ID))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != contractdomain.StatusProposed || len(loaded.Terms) != 1 || loaded.Terms[0].QuantityMilli != 10_000 {
		t.Fatalf("loaded contract = %+v", loaded)
	}
	listed, err := service.ListForHousehold(ctx, fixture.partyB)
	if err != nil || len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed contracts = %+v, %v", listed, err)
	}

	command.Terms[0].ResourceType = "not_a_resource"
	if _, err := service.Propose(ctx, command); err == nil {
		t.Fatal("expected foreign-key failure for unknown resource")
	}
	var count int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM contracts WHERE world_id = $1::uuid`, fixture.worldID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("contracts after failed proposal = %d, want 1", count)
	}
}

func createContractFixture(t *testing.T, ctx context.Context, store *postgres.Store) contractFixture {
	t.Helper()
	var fixture contractFixture
	if err := store.Pool.QueryRow(ctx, `
		INSERT INTO worlds(name, historical_start_date, current_tick, tick_duration_seconds, next_tick_at)
		VALUES ('contract integration test', DATE '0980-01-01', 5, 3600, now() + interval '1 day')
		RETURNING id::text
	`).Scan(&fixture.worldID); err != nil {
		t.Fatal(err)
	}
	var locationA, locationB string
	if err := store.Pool.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'contract a', 'farm') RETURNING id::text`, fixture.worldID).Scan(&locationA); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'contract b', 'farm') RETURNING id::text`, fixture.worldID).Scan(&locationB); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'contract party a', 0) RETURNING id::text`, fixture.worldID, locationA).Scan(&fixture.partyA); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'contract party b', 0) RETURNING id::text`, fixture.worldID, locationB).Scan(&fixture.partyB); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func removeContractFixture(t *testing.T, ctx context.Context, store *postgres.Store, worldID string) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM contract_obligations WHERE contract_id IN (SELECT id FROM contracts WHERE world_id = $1::uuid)`,
		`DELETE FROM contracts WHERE world_id = $1::uuid`,
		`DELETE FROM households WHERE world_id = $1::uuid`,
		`DELETE FROM locations WHERE world_id = $1::uuid`,
		`DELETE FROM worlds WHERE id = $1::uuid`,
	} {
		if _, err := store.Pool.Exec(ctx, statement, worldID); err != nil {
			t.Errorf("fixture cleanup: %v", err)
		}
	}
}
