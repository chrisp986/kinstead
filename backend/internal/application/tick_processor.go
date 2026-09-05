package application

import (
	"context"
	"encoding/json"
	"fmt"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	contractdomain "game/backend/internal/domain/contract"
	politicsdomain "game/backend/internal/domain/politics"
	relationshipdomain "game/backend/internal/domain/relationship"
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
	startGameDay := calendar.GameDay(world.CurrentGameDay)
	productionSeason := calendar.ProductionSeasonAt(startGameDay)
	nextGameDay, nextRemainder, err := calendar.Advance(
		startGameDay, world.CalendarRemainder,
		world.GameDaysPerTickNum, world.GameDaysPerTickDen,
	)
	if err != nil {
		return false, fmt.Errorf("advance world game day: %w", err)
	}

	// Canonical tick step 1: shipments arrive before assignments, production,
	// consumption, and fatigue are evaluated for this tick.
	if err := p.processShipmentArrivals(ctx, tx, world.ID, tick, nextGameDay); err != nil {
		return false, err
	}
	// Canonical tick step 2: obligations observe arrivals persisted by step 1.
	if err := p.processContractObligations(ctx, tx, world.ID, tick, nextGameDay); err != nil {
		return false, err
	}
	if err := p.processContractRollups(ctx, tx, world.ID); err != nil {
		return false, err
	}

	householdIDs, err := tx.ListHouseholdIDs(ctx, world.ID)
	if err != nil {
		return false, err
	}
	results := make(map[string]simulation.TickResult, len(householdIDs))
	for _, householdID := range householdIDs {
		snap, assignments, err := tx.LoadHouseholdForTick(ctx, householdID, tick)
		if err != nil {
			return false, fmt.Errorf("load household %s: %w", householdID, err)
		}
		tickContext := simulation.NeutralTickContext(simulation.Season(productionSeason))
		result, err := simulation.ProcessTick(snap.State, tick, assignments, tickContext, p.Balance)
		if err != nil {
			return false, fmt.Errorf("simulate household %s: %w", householdID, err)
		}
		if err := tx.SaveHouseholdTick(ctx, householdID, result, int64(nextGameDay)); err != nil {
			return false, fmt.Errorf("save household %s: %w", householdID, err)
		}
		results[householdID] = result
	}
	// Canonical tick step 7: resolve political events after fatigue/health.
	if err := p.processPolitics(ctx, tx, world.ID, tick, int64(nextGameDay)); err != nil {
		return false, err
	}
	// Canonical tick step 8: conservative emergency supply protection after
	// all events and political consequences have been applied.
	for _, householdID := range householdIDs {
		if err := p.processEmergencyFoodWork(ctx, tx, householdID, results[householdID], tick, int64(nextGameDay)); err != nil {
			return false, err
		}
	}

	if err := tx.FinishWorldTick(ctx, world, tick, int64(nextGameDay), nextRemainder); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit world tick: %w", err)
	}
	return true, nil
}

// processEmergencyFoodWork is deliberately narrow: only a full-capacity,
// otherwise free worker may receive one normal food-producing assignment for
// the next tick when provisions are below seven days.
func (p *TickProcessor) processEmergencyFoodWork(ctx context.Context, tx port.WorldTickTransaction, householdID string, result simulation.TickResult, tick, effectiveGameDay int64) error {
	if result.State.SupplyDays(p.Balance) >= 7 {
		return nil
	}
	for _, c := range result.State.Characters {
		if c.LaborPermille != 1000 {
			continue
		}
		activity := simulation.Fishing
		if c.Specialization == simulation.Agriculture || (c.Specialization != simulation.Fishing && result.State.FarmSpecialization == simulation.Agriculture) {
			activity = simulation.Agriculture
		}
		scheduled, err := tx.ScheduleEmergencyFoodWork(ctx, householdID, c.ID, string(activity), tick+1, tick+1, effectiveGameDay, result.State.SupplyDays(p.Balance))
		if err != nil {
			return err
		}
		if scheduled {
			break
		}
	}
	return nil
}

func (p *TickProcessor) processPolitics(ctx context.Context, tx port.WorldTickTransaction, worldID string, tick, effectiveGameDay int64) error {
	decisions, err := tx.LoadExpiringPoliticalDecisions(ctx, worldID, tick)
	if err != nil {
		return fmt.Errorf("load expiring political demands: %w", err)
	}
	for _, d := range decisions {
		terms, err := politicalTerms(d.Parameters, politicsdomain.DemandType(d.EventType))
		if err != nil {
			return err
		}
		resolution, err := politicsdomain.ResolveChoiceWithTerms(politicsdomain.DemandType(d.EventType), politicsdomain.OptionRefuse, terms)
		if err != nil {
			return err
		}
		changed, err := tx.AutoResolvePoliticalDecision(ctx, d, tick, string(resolution.Option), resolution.StandingDelta)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		if err := tx.ApplyPoliticalScoreDelta(ctx, d.WorldID, d.HouseholdID, d.PoliticalActorID, resolution.StandingDelta); err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]any{"actor_id": d.PoliticalActorID, "demand_type": d.EventType, "selected_option": resolution.Option, "standing_delta": resolution.StandingDelta, "deadline_tick": d.ExpiresTick, "deadline_game_day": d.ExpiresGameDay})
		if err := tx.InsertPoliticalChronicle(ctx, d.HouseholdID, tick, effectiveGameDay, "political_demand_auto_resolved", d.ID, d.PoliticalActorID, "", data); err != nil {
			return err
		}
	}
	events, err := tx.LoadPoliticalEventsStartingTick(ctx, worldID, tick)
	if err != nil {
		return fmt.Errorf("load political demands: %w", err)
	}
	for _, event := range events {
		if event.ExpiresTick <= event.StartsTick {
			return fmt.Errorf("political event %s has invalid deadline", event.ID)
		}
		households, err := tx.ListHouseholdsForPoliticalEvent(ctx, event.ID)
		if err != nil {
			return err
		}
		for _, householdID := range households {
			terms := politicsdomain.DefaultTerms(politicsdomain.DemandType(event.EventType))
			encoded, _ := json.Marshal(terms)
			d := port.PoliticalDecisionRecord{HouseholdID: householdID, WorldID: worldID, WorldEventID: event.ID, DecisionType: event.EventType, AvailableFromTick: event.StartsTick, ExpiresTick: event.ExpiresTick, AvailableFromGameDay: event.StartsGameDay, ExpiresGameDay: event.ExpiresGameDay, Parameters: encoded}
			created, err := tx.InsertPoliticalDecision(ctx, d)
			if err != nil {
				return err
			}
			if !created {
				continue
			}
			data, _ := json.Marshal(map[string]any{"actor_id": event.PoliticalActorID, "demand_type": event.EventType, "deadline_tick": event.ExpiresTick, "deadline_game_day": event.ExpiresGameDay})
			if err := tx.InsertPoliticalReceivedChronicle(ctx, householdID, tick, effectiveGameDay, event.ID, event.PoliticalActorID, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *TickProcessor) processContractObligations(ctx context.Context, tx port.WorldTickTransaction, worldID string, tick int64, gameDay calendar.GameDay) error {
	assessments, err := tx.LoadContractObligationsForTick(ctx, worldID, tick, int64(gameDay))
	if err != nil {
		return fmt.Errorf("load contract obligations: %w", err)
	}
	for _, assessment := range assessments {
		var updated contractdomain.Obligation
		if assessment.GameDaySchedule {
			updated, err = assessment.Obligation.AssessGameDay(contractdomain.GameDay(gameDay), assessment.ActualArrivalGameDay)
		} else {
			updated, err = assessment.Obligation.Assess(contractdomain.Tick(tick), assessment.ActualArrivalTick)
		}
		if err != nil {
			return fmt.Errorf("assess contract obligation %s: %w", assessment.Obligation.ID, err)
		}
		// Keep the legacy fulfillment column synchronized while the database
		// still enforces the v0.3 state constraint. Outcome classification is
		// based only on the game-day snapshot above.
		if assessment.ActualArrivalTick != nil && assessment.GameDaySchedule {
			fulfilledTick := contractdomain.Tick(*assessment.ActualArrivalTick)
			updated.FulfilledTick = &fulfilledTick
		}
		if updated.Status == assessment.Obligation.Status &&
			((assessment.GameDaySchedule && equalContractGameDay(updated.FulfilledGameDay, assessment.Obligation.FulfilledGameDay)) ||
				(!assessment.GameDaySchedule && equalContractTick(updated.FulfilledTick, assessment.Obligation.FulfilledTick))) {
			continue
		}
		var event *relationshipdomain.Event
		if assessment.GameDaySchedule {
			event, err = relationshipdomain.ContractOutcomeGameDay(assessment.WorldID, assessment.Obligation, updated, contractdomain.GameDay(gameDay), contractdomain.Tick(tick))
		} else {
			event, err = relationshipdomain.ContractOutcome(assessment.WorldID, assessment.Obligation, updated, contractdomain.Tick(tick))
		}
		if err != nil {
			return fmt.Errorf("derive relationship outcome for obligation %s: %w", assessment.Obligation.ID, err)
		}
		persisted, err := tx.PersistContractObligationAssessment(ctx, assessment.Obligation, updated, event)
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

func equalContractGameDay(a, b *contractdomain.GameDay) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func politicalTerms(data []byte, demand politicsdomain.DemandType) (politicsdomain.DemandTerms, error) {
	terms := politicsdomain.DefaultTerms(demand)
	if len(data) != 0 && string(data) != "{}" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return politicsdomain.DemandTerms{}, fmt.Errorf("decode political demand terms: %w", err)
		}
		required := []string{"honor_standing_delta", "refuse_standing_delta"}
		if demand == politicsdomain.DemandLaborService {
			required = append(required, "service_ticks")
		} else if demand == politicsdomain.DemandLevy {
			required = append(required, "wood_cost_milli", "silver_cost_milli")
		}
		for _, key := range required {
			if _, ok := raw[key]; !ok {
				return politicsdomain.DemandTerms{}, fmt.Errorf("missing political demand term %q", key)
			}
		}
		if err := json.Unmarshal(data, &terms); err != nil {
			return politicsdomain.DemandTerms{}, fmt.Errorf("decode political demand terms: %w", err)
		}
	}
	if err := terms.Validate(demand); err != nil {
		return politicsdomain.DemandTerms{}, err
	}
	return terms, nil
}

func (p *TickProcessor) processContractRollups(ctx context.Context, tx port.WorldTickTransaction, worldID string) error {
	snapshots, err := tx.LoadActiveContractsForRollup(ctx, worldID)
	if err != nil {
		return fmt.Errorf("load contract rollups: %w", err)
	}
	for _, snapshot := range snapshots {
		updated, err := snapshot.Contract.RollUp(snapshot.Obligations)
		if err != nil {
			return fmt.Errorf("roll up contract %s: %w", snapshot.Contract.ID, err)
		}
		if updated.Status == snapshot.Contract.Status {
			continue
		}
		persisted, err := tx.PersistContractRollup(ctx, snapshot.Contract, updated)
		if err != nil {
			return fmt.Errorf("persist contract %s rollup: %w", snapshot.Contract.ID, err)
		}
		if !persisted {
			return fmt.Errorf("contract %s changed during tick", snapshot.Contract.ID)
		}
	}
	return nil
}

func (p *TickProcessor) processShipmentArrivals(ctx context.Context, tx port.WorldTickTransaction, worldID string, tick int64, gameDay calendar.GameDay) error {
	due, err := tx.LoadDueShipments(ctx, worldID, tick)
	if err != nil {
		return fmt.Errorf("load shipment arrivals: %w", err)
	}
	for _, value := range due {
		arrived, err := value.ArriveAt(shipmentdomain.Tick(tick), shipmentdomain.GameDay(gameDay))
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
