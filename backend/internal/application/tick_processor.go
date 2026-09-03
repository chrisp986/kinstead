package application

import (
	"context"
	"fmt"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	contractdomain "game/backend/internal/domain/contract"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

type TickProcessor struct {
	Store   port.TickRepository
	Balance simulation.BalanceConfig
}

func NewTickProcessor(store port.TickRepository) *TickProcessor {
	return &TickProcessor{Store: store, Balance: balance.V03()}
}

// ProcessOneDueWorld atomically advances at most one due world by exactly one tick.
// It returns false when no world is currently due.
func (p *TickProcessor) ProcessOneDueWorld(ctx context.Context) (bool, error) {
	tx, err := p.Store.BeginWorldTick(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	world, ok, err := tx.ClaimDueWorld(ctx)
	if err != nil || !ok {
		return ok, err
	}

	tick := world.CurrentTick + 1
	processed, err := tx.IsTickProcessed(ctx, world.ID, tick)
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
	// Canonical tick step 2: obligations observe arrivals persisted by step 1.
	if err := p.processContractObligations(ctx, tx, world.ID, tick); err != nil {
		return false, err
	}

	householdIDs, err := tx.ListHouseholdIDs(ctx, world.ID)
	if err != nil {
		return false, err
	}
	for _, householdID := range householdIDs {
		snap, assignments, err := tx.LoadHouseholdForTick(ctx, householdID, tick)
		if err != nil {
			return false, fmt.Errorf("load household %s: %w", householdID, err)
		}
		historicalDate, err := (calendar.Clock{
			StartDate:   snap.HistoricalStart,
			DaysPerTick: calendar.Rational{Numerator: int64(snap.HistoricalDaysPerTickNum), Denominator: int64(snap.HistoricalDaysPerTickDen)},
		}).DateAtTick(tick)
		if err != nil {
			return false, fmt.Errorf("historical date for world %s tick %d: %w", world.ID, tick, err)
		}
		tickContext := simulation.NeutralTickContext(simulation.SeasonForDate(historicalDate))
		result, err := simulation.ProcessTick(snap.State, tick, assignments, tickContext, p.Balance)
		if err != nil {
			return false, fmt.Errorf("simulate household %s: %w", householdID, err)
		}
		if err := tx.SaveHouseholdTick(ctx, householdID, result); err != nil {
			return false, fmt.Errorf("save household %s: %w", householdID, err)
		}
	}

	if err := tx.FinishWorldTick(ctx, world, tick); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit world tick: %w", err)
	}
	return true, nil
}

func (p *TickProcessor) processContractObligations(ctx context.Context, tx port.WorldTickTransaction, worldID string, tick int64) error {
	assessments, err := tx.LoadContractObligationsForTick(ctx, worldID, tick)
	if err != nil {
		return fmt.Errorf("load contract obligations: %w", err)
	}
	for _, assessment := range assessments {
		updated, err := assessment.Obligation.Assess(contractdomain.Tick(tick), assessment.ActualArrivalTick)
		if err != nil {
			return fmt.Errorf("assess contract obligation %s: %w", assessment.Obligation.ID, err)
		}
		if updated.Status == assessment.Obligation.Status && equalContractTick(updated.FulfilledTick, assessment.Obligation.FulfilledTick) {
			continue
		}
		persisted, err := tx.PersistContractObligationAssessment(ctx, assessment.Obligation, updated)
		if err != nil {
			return fmt.Errorf("persist contract obligation %s: %w", assessment.Obligation.ID, err)
		}
		if !persisted {
			return fmt.Errorf("contract obligation %s changed during tick", assessment.Obligation.ID)
		}
	}
	return nil
}

func equalContractTick(a, b *contractdomain.Tick) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func (p *TickProcessor) processShipmentArrivals(ctx context.Context, tx port.WorldTickTransaction, worldID string, tick int64) error {
	due, err := tx.LoadDueShipments(ctx, worldID, tick)
	if err != nil {
		return fmt.Errorf("load shipment arrivals: %w", err)
	}
	for _, value := range due {
		arrived, err := value.Arrive(shipmentdomain.Tick(tick))
		if err != nil {
			return fmt.Errorf("arrive shipment %s: %w", value.ID, err)
		}
		persisted, err := tx.PersistShipmentArrival(ctx, arrived)
		if err != nil {
			return fmt.Errorf("persist shipment %s arrival: %w", value.ID, err)
		}
		if !persisted {
			return fmt.Errorf("shipment %s arrival was already persisted", value.ID)
		}
	}
	return nil
}
