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
	if _, err := service.Respond(ctx, RespondContractCommand{
		ContractID: string(created.ID), CounterpartyHouseholdID: fixture.partyA, Accept: true,
	}); err != ErrContractResponseForbidden {
		t.Fatalf("unauthorized response error = %v", err)
	}
	responseResults := make(chan struct {
		value contractdomain.Contract
		err   error
	}, 2)
	for range 2 {
		go func() {
			value, err := service.Respond(ctx, RespondContractCommand{
				ContractID: string(created.ID), CounterpartyHouseholdID: fixture.partyB, Accept: true,
			})
			responseResults <- struct {
				value contractdomain.Contract
				err   error
			}{value: value, err: err}
		}()
	}
	var responseErrors []error
	for range 2 {
		result := <-responseResults
		if result.err != nil {
			responseErrors = append(responseErrors, result.err)
			continue
		}
		if result.value.Status != contractdomain.StatusActive {
			t.Fatalf("concurrent acceptance = %+v", result.value)
		}
	}
	for _, responseErr := range responseErrors {
		if _, err := service.Respond(ctx, RespondContractCommand{
			ContractID: string(created.ID), CounterpartyHouseholdID: fixture.partyB, Accept: true,
		}); err != nil {
			t.Fatalf("retry after concurrent response error %v: %v", responseErr, err)
		}
	}
	obligations, err := service.ListObligations(ctx, string(created.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(obligations) != 3 || obligations[0].DueArrivalTick != 7 || obligations[1].DueArrivalTick != 10 || obligations[2].DueArrivalTick != 13 {
		t.Fatalf("obligations = %+v", obligations)
	}
	if _, err := service.Respond(ctx, RespondContractCommand{
		ContractID: string(created.ID), CounterpartyHouseholdID: fixture.partyB, Accept: true,
	}); err != nil {
		t.Fatalf("idempotent acceptance: %v", err)
	}
	replayed, err := service.ListObligations(ctx, string(created.ID))
	if err != nil || len(replayed) != len(obligations) {
		t.Fatalf("obligations after retry = %+v, %v", replayed, err)
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
