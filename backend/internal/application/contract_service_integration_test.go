//go:build postgres

package application

import (
	"context"
	"errors"
	"os"
	"testing"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/postgres"
)

type contractFixture struct {
	worldID   string
	partyA    string
	partyB    string
	locationA string
	locationB string
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
		StartGameDay: 45, EndGameDay: 73, IntervalDays: 14,
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
	if len(obligations) != 3 || obligations[0].DueArrivalTick != 6 || obligations[1].DueArrivalTick != 8 || obligations[2].DueArrivalTick != 10 ||
		obligations[0].DueGameDay != 45 || obligations[1].DueGameDay != 59 || obligations[2].DueGameDay != 73 {
		t.Fatalf("obligations = %+v", obligations)
	}
	if _, err := service.DispatchObligation(ctx, DispatchContractObligationCommand{
		ObligationID: string(obligations[0].ID), DebtorHouseholdID: fixture.partyB,
	}); err != ErrContractDispatchForbidden {
		t.Fatalf("unauthorized dispatch error = %v", err)
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
	dispatchResults := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := service.DispatchObligation(ctx, DispatchContractObligationCommand{
				ObligationID: string(obligations[0].ID), DebtorHouseholdID: fixture.partyA,
			})
			dispatchResults <- err
		}()
	}
	var dispatchErrors []error
	for range 2 {
		if err := <-dispatchResults; err != nil {
			dispatchErrors = append(dispatchErrors, err)
		}
	}
	for _, dispatchErr := range dispatchErrors {
		if _, err := service.DispatchObligation(ctx, DispatchContractObligationCommand{
			ObligationID: string(obligations[0].ID), DebtorHouseholdID: fixture.partyA,
		}); err != nil {
			t.Fatalf("retry after concurrent dispatch error %v: %v", dispatchErr, err)
		}
	}
	assertContractDispatch(t, ctx, store, obligations[0], fixture)
	if _, err := store.Pool.Exec(ctx, `
		UPDATE resource_stocks SET quantity_milli = 0
		WHERE household_id = $1::uuid AND resource_code = 'provisions'
	`, fixture.partyA); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DispatchObligation(ctx, DispatchContractObligationCommand{
		ObligationID: string(obligations[1].ID), DebtorHouseholdID: fixture.partyA,
	}); !errors.Is(err, postgres.ErrInsufficientResources) {
		t.Fatalf("insufficient-stock dispatch error = %v", err)
	}
	var failedShipmentID *string
	if err := store.Pool.QueryRow(ctx, `SELECT shipment_id::text FROM contract_obligations WHERE id = $1::uuid`, obligations[1].ID).Scan(&failedShipmentID); err != nil {
		t.Fatal(err)
	}
	if failedShipmentID != nil {
		t.Fatalf("failed dispatch linked shipment %s", *failedShipmentID)
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
		INSERT INTO worlds(name, historical_start_date, current_tick, current_game_day, calendar_remainder, tick_duration_seconds, next_tick_at)
		VALUES ('contract integration test', DATE '0980-01-01', 5, (5 * 91) / 12, (5 * 91) % 12, 3600, now() + interval '1 day')
		RETURNING id::text
	`).Scan(&fixture.worldID); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'contract a', 'farm') RETURNING id::text`, fixture.worldID).Scan(&fixture.locationA); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'contract b', 'farm') RETURNING id::text`, fixture.worldID).Scan(&fixture.locationB); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'contract party a', 0) RETURNING id::text`, fixture.worldID, fixture.locationA).Scan(&fixture.partyA); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'contract party b', 0) RETURNING id::text`, fixture.worldID, fixture.locationB).Scan(&fixture.partyB); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `
		INSERT INTO location_routes(world_id, origin_location_id, destination_location_id, distance_class)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'local')
	`, fixture.worldID, fixture.locationA, fixture.locationB); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `
		INSERT INTO resource_stocks(household_id, resource_code, quantity_milli)
		VALUES ($1::uuid, 'provisions', 30000)
	`, fixture.partyA); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func assertContractDispatch(t *testing.T, ctx context.Context, store *postgres.Store, obligation contractdomain.Obligation, fixture contractFixture) {
	t.Helper()
	var shipmentCount, chronicleCount int
	var stock, departureTick, arrivalTick, transportCost int64
	var shipmentID, status string
	if err := store.Pool.QueryRow(ctx, `
		SELECT count(*) OVER(), id::text, departure_tick, expected_arrival_tick, transport_cost_milli
		FROM shipments WHERE world_id = $1::uuid
	`, fixture.worldID).Scan(&shipmentCount, &shipmentID, &departureTick, &arrivalTick, &transportCost); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `
		SELECT status FROM contract_obligations WHERE id = $1::uuid AND shipment_id = $2::uuid
	`, obligation.ID, shipmentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `
		SELECT quantity_milli FROM resource_stocks WHERE household_id = $1::uuid AND resource_code = 'provisions'
	`, fixture.partyA).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `
		SELECT count(*) FROM chronicle_entries
		WHERE related_contract_id = $1::uuid AND related_shipment_id = $2::uuid
	`, obligation.ContractID, shipmentID).Scan(&chronicleCount); err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 1 || departureTick != 5 || arrivalTick != 7 || transportCost != 1_000 ||
		status != "dispatched" || stock != 20_000 || chronicleCount != 2 {
		t.Fatalf("dispatch count/departure/arrival/cost/status/stock/chronicle = %d/%d/%d/%d/%s/%d/%d",
			shipmentCount, departureTick, arrivalTick, transportCost, status, stock, chronicleCount)
	}
}

func removeContractFixture(t *testing.T, ctx context.Context, store *postgres.Store, worldID string) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM chronicle_entries WHERE household_id IN (SELECT id FROM households WHERE world_id = $1::uuid)`,
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
