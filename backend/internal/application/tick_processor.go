//go:build postgres

package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/postgres"
	"game/backend/internal/simulation"
)

type TickProcessor struct {
	Store   *postgres.Store
	Balance simulation.BalanceConfig
}

func NewTickProcessor(store *postgres.Store) *TickProcessor {
	return &TickProcessor{Store: store, Balance: simulation.DefaultBalanceConfig()}
}

// ProcessOneDueWorld atomically advances at most one due world by exactly one tick.
// It returns false when no world is currently due.
func (p *TickProcessor) ProcessOneDueWorld(ctx context.Context) (bool, error) {
	tx, err := p.Store.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	world, ok, err := p.Store.ClaimDueWorld(ctx, tx)
	if err != nil || !ok {
		return ok, err
	}

	tick := world.CurrentTick + 1
	processed, err := p.Store.IsTickProcessed(ctx, tx, world.ID, tick)
	if err != nil {
		return false, err
	}
	if processed {
		return false, fmt.Errorf("world %s tick %d already processed while current_tick is %d", world.ID, tick, world.CurrentTick)
	}

	// Canonical tick step 1: shipments arrive before assignments, production,
	// consumption, and fatigue are evaluated for this tick.
	if err := p.processShipmentArrivals(ctx, tx, world.ID, tick); err != nil {
		return false, err
	}

	householdIDs, err := p.Store.ListHouseholdIDs(ctx, tx, world.ID)
	if err != nil {
		return false, err
	}
	for _, householdID := range householdIDs {
		snap, assignments, err := p.Store.LoadHouseholdForTick(ctx, tx, householdID, tick)
		if err != nil {
			return false, fmt.Errorf("load household %s: %w", householdID, err)
		}
		result, err := simulation.ProcessTick(snap.State, tick, assignments, p.Balance)
		if err != nil {
			return false, fmt.Errorf("simulate household %s: %w", householdID, err)
		}
		if err := p.Store.SaveHouseholdTick(ctx, tx, householdID, result); err != nil {
			return false, fmt.Errorf("save household %s: %w", householdID, err)
		}
	}

	if err := p.Store.FinishWorldTick(ctx, tx, world, tick); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxCommitRollback) {
			return false, fmt.Errorf("serializable tick conflict: %w", err)
		}
		return false, err
	}
	return true, nil
}

func (p *TickProcessor) processShipmentArrivals(ctx context.Context, tx pgx.Tx, worldID string, tick int64) error {
	due, err := p.Store.LoadDueShipments(ctx, tx, worldID, tick)
	if err != nil {
		return fmt.Errorf("load shipment arrivals: %w", err)
	}
	for _, value := range due {
		arrived, err := value.Arrive(shipmentdomain.Tick(tick))
		if err != nil {
			return fmt.Errorf("arrive shipment %s: %w", value.ID, err)
		}
		persisted, err := p.Store.PersistShipmentArrival(ctx, tx, arrived)
		if err != nil {
			return fmt.Errorf("persist shipment %s arrival: %w", value.ID, err)
		}
		if !persisted {
			return fmt.Errorf("shipment %s arrival was already persisted", value.ID)
		}
	}
	return nil
}
