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

func tickFailure(world port.WorldClaim, tick int64, stage, householdID string, err error) error {
	return &TickFailure{WorldID: world.ID, HouseholdID: householdID, Tick: tick, Stage: stage, Err: err}
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
		return false, tickFailure(world, tick, "claim_world", "", fmt.Errorf("tick already processed while current_tick is %d", world.CurrentTick))
	}
	startGameDay := calendar.GameDay(world.CurrentGameDay)
	if !world.SimulationModel.Valid() {
		return false, fmt.Errorf("unsupported simulation model %q", world.SimulationModel)
	}
	historicalDefinition, err := calendar.DefinitionForModel(string(world.SimulationModel))
	if err != nil {
		return false, err
	}
	productionSeason := historicalDefinition.ProductionSeasonAt(startGameDay)
	nextGameDay, nextRemainder, err := calendar.Advance(
		startGameDay, world.CalendarRemainder,
		world.GameDaysPerTickNum, world.GameDaysPerTickDen,
	)
	if err != nil {
		return false, fmt.Errorf("advance world game day: %w", err)
	}
	householdIDs, err := tx.ListHouseholdIDs(ctx, world.ID)
	if err != nil {
		return false, fmt.Errorf("list households: %w", err)
	}
	opening := make(map[string]port.HouseholdAccountingState, len(householdIDs))
	for _, householdID := range householdIDs {
		state, err := tx.LoadHouseholdAccounting(ctx, householdID)
		if err != nil {
			return false, fmt.Errorf("capture opening accounting for household %s: %w", householdID, err)
		}
		opening[householdID] = state
	}

	// Canonical tick step 1: shipments arrive before assignments, production,
	// consumption, and fatigue are evaluated for this tick.
	arrivals, err := p.processShipmentArrivals(ctx, tx, world.ID, tick, nextGameDay)
	if err != nil {
		return false, tickFailure(world, tick, "shipments", "", err)
	}
	// Canonical tick step 2: obligations observe arrivals persisted by step 1.
	if err := p.processContractObligations(ctx, tx, world.ID, tick, nextGameDay); err != nil {
		return false, tickFailure(world, tick, "contracts", "", err)
	}
	if err := p.processContractRollups(ctx, tx, world.ID); err != nil {
		return false, tickFailure(world, tick, "contracts", "", err)
	}

	results := make(map[string]simulation.TickResult, len(householdIDs))
	hourResults := make(map[string]simulation.HourResult, len(householdIDs))
	for _, householdID := range householdIDs {
		snap, assignments, err := tx.LoadHouseholdForTick(ctx, householdID, tick)
		if err != nil {
			return false, tickFailure(world, tick, "household_simulation", householdID, fmt.Errorf("load household: %w", err))
		}
		if world.SimulationModel.UsesHourlyLabor() {
			if snap.DailyLabor == nil {
				return false, fmt.Errorf("daily-labor household %s has no daily state", householdID)
			}
			start, err := calendar.MomentAtClock(calendar.ClockState{Day: startGameDay, Remainder: world.CalendarRemainder, GameDaysPerTickNum: world.GameDaysPerTickNum, GameDaysPerTickDen: world.GameDaysPerTickDen})
			if err != nil {
				return false, fmt.Errorf("resolve daily clock: %w", err)
			}
			workContext := simulation.DailyLaborWorkContext(start.Day)
			hourlyBalance := balance.DailyLaborV1()
			if world.SimulationModel == port.ModelMonthlySeasons {
				if world.CalendarAnchorAt == nil || world.WorldUTCOffsetMinutes == nil {
					return false, fmt.Errorf("monthly world %s has no scheduling anchor", world.ID)
				}
				date, dateErr := calendar.SchedulingDate(*world.CalendarAnchorAt, *world.WorldUTCOffsetMinutes, start)
				if dateErr != nil {
					return false, fmt.Errorf("resolve monthly scheduling date: %w", dateErr)
				}
				position, positionErr := calendar.SeasonalPositionForDate(date)
				if positionErr != nil {
					return false, positionErr
				}
				workday, workdayErr := calendar.WorkdayForDate(date, calendar.DefaultDaylightConfig())
				if workdayErr != nil {
					return false, workdayErr
				}
				workContext = simulation.WorkContext{Season: position.Season, Workday: workday}
				hourlyBalance = balance.MonthlySeasonsV1()
			}
			hour, err := simulation.ProcessHourWithContext(*snap.DailyLabor, simulation.HourInterval{Start: start, End: calendar.AdvanceMoment(start, 1)}, workContext, hourlyBalance)
			if err != nil {
				return false, tickFailure(world, tick, "household_simulation", householdID, fmt.Errorf("simulate daily household: %w", err))
			}
			if err := tx.SaveHouseholdDailyTick(ctx, householdID, hour); err != nil {
				return false, tickFailure(world, tick, "persistence", householdID, fmt.Errorf("save daily household: %w", err))
			}
			hourResults[householdID] = hour
			continue
		}
		tickContext := simulation.NeutralTickContext(simulation.Season(productionSeason))
		tickContext.GameDaysPerTickNum = world.GameDaysPerTickNum
		tickContext.GameDaysPerTickDen = world.GameDaysPerTickDen
		result, err := simulation.ProcessTick(snap.State, tick, assignments, tickContext, p.Balance)
		if err != nil {
			return false, tickFailure(world, tick, "household_simulation", householdID, fmt.Errorf("simulate household: %w", err))
		}
		if err := tx.SaveHouseholdTick(ctx, householdID, result, int64(nextGameDay)); err != nil {
			return false, tickFailure(world, tick, "persistence", householdID, fmt.Errorf("save household: %w", err))
		}
		results[householdID] = result
	}
	// Canonical tick step 7: resolve political events after fatigue/health.
	if err := p.processPolitics(ctx, tx, world.ID, tick, int64(nextGameDay)); err != nil {
		return false, tickFailure(world, tick, "politics", "", err)
	}
	// Canonical tick step 8: conservative emergency supply protection after
	// all events and political consequences have been applied.
	for _, householdID := range householdIDs {
		if world.SimulationModel.UsesHourlyLabor() {
			continue
		}
		if err := p.processEmergencyFoodWork(ctx, tx, householdID, results[householdID], tick, int64(nextGameDay), world.GameDaysPerTickNum, world.GameDaysPerTickDen); err != nil {
			return false, tickFailure(world, tick, "emergency_ai", householdID, err)
		}
	}
	for _, householdID := range householdIDs {
		closing, err := tx.LoadHouseholdAccounting(ctx, householdID)
		if err != nil {
			return false, tickFailure(world, tick, "persistence", householdID, fmt.Errorf("capture closing accounting: %w", err))
		}
		diagnostic, err := buildTickDiagnostic(world, householdID, tick, startGameDay, nextGameDay, opening[householdID], closing, results[householdID], hourResults[householdID], p.Balance, arrivals[householdID])
		if err != nil {
			return false, tickFailure(world, tick, "persistence", householdID, err)
		}
		if err := tx.PersistHouseholdTickDiagnostic(ctx, diagnostic); err != nil {
			return false, tickFailure(world, tick, "persistence", householdID, fmt.Errorf("persist diagnostic: %w", err))
		}
	}

	if err := tx.FinishWorldTick(ctx, world, tick, int64(nextGameDay), nextRemainder); err != nil {
		return false, tickFailure(world, tick, "commit", "", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, tickFailure(world, tick, "commit", "", fmt.Errorf("commit world tick: %w", err))
	}
	return true, nil
}

func buildTickDiagnostic(world port.WorldClaim, householdID string, tick int64, startDay, endDay calendar.GameDay, opening, closing port.HouseholdAccountingState, legacy simulation.TickResult, hourly simulation.HourResult, cfg simulation.BalanceConfig, arrivals map[string]int64) (port.AdminTickDiagnostic, error) {
	details := map[string]any{
		"coverage":                "recorded gameplay movements; politics and non-stock stages are not represented as resource adjustments",
		"accounting_status":       "partial",
		"opening_stored_milli":    opening.StoredMilli,
		"closing_stored_milli":    closing.StoredMilli,
		"opening_pending_milli":   opening.PendingOutputMilli,
		"closing_pending_milli":   closing.PendingOutputMilli,
		"incoming_arrivals_milli": nonNilInt64Map(arrivals),
	}
	var characters []simulation.CharacterDiagnostic
	if world.SimulationModel.UsesHourlyLabor() {
		details["earned_production_milli"] = map[string]int64{"provisions": hourly.ProducedProvisionsMilli, "wood": hourly.ProducedWoodMilli}
		details["deposited_production_milli"] = map[string]int64{"provisions": hourly.SettledProvisionsMilli, "wood": hourly.SettledWoodMilli}
		details["actual_consumption_milli"] = map[string]int64{"provisions": hourly.ConsumedProvisionsMilli, "wood": hourly.ConsumedWoodMilli}
		details["requested_consumption_milli"] = map[string]int64{"provisions": hourly.ConsumedProvisionsMilli + hourly.FoodShortageMilli, "wood": hourly.ConsumedWoodMilli + hourly.WoodShortageMilli}
		characters = hourly.CharacterDiagnostics
	} else {
		actualFood := cfg.ConsumptionPerTickMilli - legacy.FoodShortageMilli
		if actualFood < 0 {
			actualFood = 0
		}
		details["direct_production_milli"] = map[string]int64{"provisions": legacy.ProducedProvisionsMilli, "wood": legacy.ProducedWoodMilli}
		actualWood := cfg.DailyWoodUpkeepMilli - legacy.WoodShortageMilli
		if actualWood < 0 {
			actualWood = 0
		}
		details["actual_consumption_milli"] = map[string]int64{"provisions": actualFood, "wood": actualWood}
		details["requested_consumption_milli"] = map[string]int64{"provisions": cfg.ConsumptionPerTickMilli, "wood": cfg.DailyWoodUpkeepMilli}
		characters = legacy.CharacterDiagnostics
	}
	if characters == nil {
		characters = []simulation.CharacterDiagnostic{}
	}
	details["characters"] = characters
	arrivalsForResource := nonNilInt64Map(arrivals)
	expectedStored := copyInt64Map(opening.StoredMilli)
	if world.SimulationModel.UsesHourlyLabor() {
		deposits := map[string]int64{"provisions": hourly.SettledProvisionsMilli, "wood": hourly.SettledWoodMilli}
		actual := map[string]int64{"provisions": hourly.ConsumedProvisionsMilli, "wood": hourly.ConsumedWoodMilli}
		for resource, amount := range arrivalsForResource {
			expectedStored[resource] += amount
		}
		for resource, amount := range deposits {
			expectedStored[resource] += amount
		}
		for resource, amount := range actual {
			expectedStored[resource] -= amount
		}
		expectedPending := map[string]int64{"provisions": opening.PendingOutputMilli["provisions"] + hourly.ProducedProvisionsMilli - hourly.SettledProvisionsMilli, "wood": opening.PendingOutputMilli["wood"] + hourly.ProducedWoodMilli - hourly.SettledWoodMilli}
		details["expected_closing_pending_milli"] = expectedPending
		if !sameInt64MapValues(expectedPending, closing.PendingOutputMilli) {
			return port.AdminTickDiagnostic{}, fmt.Errorf("pending accounting mismatch for household %s", householdID)
		}
	} else {
		for resource, amount := range arrivalsForResource {
			expectedStored[resource] += amount
		}
		expectedStored["provisions"] += legacy.ProducedProvisionsMilli - (cfg.ConsumptionPerTickMilli - legacy.FoodShortageMilli)
		expectedStored["wood"] += legacy.ProducedWoodMilli - (cfg.DailyWoodUpkeepMilli - legacy.WoodShortageMilli)
	}
	details["expected_closing_stored_milli"] = expectedStored
	if !sameInt64MapValues(expectedStored, closing.StoredMilli) {
		return port.AdminTickDiagnostic{}, fmt.Errorf("stored accounting mismatch for household %s", householdID)
	}
	details["tracked_balance_check"] = "passed"
	value := port.AdminTickDiagnostic{WorldID: world.ID, HouseholdID: householdID, Tick: tick, IntervalStartDay: int64(startDay), IntervalEndDay: int64(endDay), SimulationModel: world.SimulationModel, DiagnosticSchemaVersion: 1, Details: details}
	if world.SimulationModel.UsesHourlyLabor() {
		if moment, err := calendar.MomentAtClock(calendar.ClockState{Day: startDay, Remainder: world.CalendarRemainder, GameDaysPerTickNum: world.GameDaysPerTickNum, GameDaysPerTickDen: world.GameDaysPerTickDen}); err == nil {
			end := calendar.AdvanceMoment(moment, 1)
			value.IntervalStartHour = &moment.Hour
			value.IntervalEndDay = int64(end.Day)
			value.IntervalEndHour = &end.Hour
		}
	}
	return value, nil
}

func nonNilInt64Map(value map[string]int64) map[string]int64 {
	if value == nil {
		return map[string]int64{}
	}
	return value
}

func copyInt64Map(value map[string]int64) map[string]int64 {
	copyValue := make(map[string]int64, len(value))
	for key, amount := range value {
		copyValue[key] = amount
	}
	return copyValue
}

func sameInt64MapValues(expected, actual map[string]int64) bool {
	keys := map[string]struct{}{}
	for key := range expected {
		keys[key] = struct{}{}
	}
	for key := range actual {
		keys[key] = struct{}{}
	}
	for key := range keys {
		if expected[key] != actual[key] {
			return false
		}
	}
	return true
}

// processEmergencyFoodWork is deliberately narrow: only an available,
// sufficiently rested worker may receive one normal food-producing assignment
// for the next tick when provisions are below seven days.
func (p *TickProcessor) processEmergencyFoodWork(ctx context.Context, tx port.WorldTickTransaction, householdID string, result simulation.TickResult, tick, effectiveGameDay, gameDaysPerTickNum, gameDaysPerTickDen int64) error {
	foodContext, err := tx.LoadEmergencyFoodContext(ctx, householdID, tick+1)
	if err != nil {
		return err
	}
	decision := EvaluateEmergencyFoodPolicy(EmergencyFoodPolicyInput{
		State: result.State, CurrentTick: tick, CurrentGameDay: effectiveGameDay,
		GameDaysPerTickNum: gameDaysPerTickNum, GameDaysPerTickDen: gameDaysPerTickDen,
		Season:      simulation.Season(calendar.ProductionSeasonAt(calendar.GameDay(effectiveGameDay))),
		Assignments: foodContext.Assignments, IncomingShipments: foodContext.IncomingShipments, Balance: p.Balance,
	})
	record := port.EmergencyFoodDecisionRecord{
		CharacterID: decision.CharacterID, Activity: string(decision.Activity), StartsTick: tick + 1, EndsTick: tick + 1,
		OccurredTick: tick, OccurredGameDay: effectiveGameDay, Reason: decision.Reason, SupplyGameDays: decision.SupplyGameDays,
		ExpectedProductionMilli: decision.ExpectedProductionMilli, RemainsAtRisk: decision.RemainsAtRisk,
	}
	if decision.Schedule {
		_, err = tx.ScheduleEmergencyFoodWork(ctx, householdID, record)
		return err
	}
	return tx.RecordEmergencyFoodDecision(ctx, householdID, record)
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
			if _, ticks := raw["service_ticks"]; !ticks {
				if _, hours := raw["service_hours"]; !hours {
					return politicsdomain.DemandTerms{}, fmt.Errorf("missing political demand term %q", "service_hours")
				}
			}
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
		if demand == politicsdomain.DemandLaborService {
			if _, ok := raw["service_hours"]; !ok {
				terms.ServiceHours = 0 // legacy terms retain their tick duration
			}
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

func (p *TickProcessor) processShipmentArrivals(ctx context.Context, tx port.WorldTickTransaction, worldID string, tick int64, gameDay calendar.GameDay) (map[string]map[string]int64, error) {
	due, err := tx.LoadDueShipments(ctx, worldID, tick)
	if err != nil {
		return nil, fmt.Errorf("load shipment arrivals: %w", err)
	}
	arrivals := make(map[string]map[string]int64)
	for _, value := range due {
		arrived, err := value.ArriveAt(shipmentdomain.Tick(tick), shipmentdomain.GameDay(gameDay))
		if err != nil {
			return nil, fmt.Errorf("arrive shipment %s: %w", value.ID, err)
		}
		persisted, err := tx.PersistShipmentArrival(ctx, arrived)
		if err != nil {
			return nil, fmt.Errorf("persist shipment %s arrival: %w", value.ID, err)
		}
		if !persisted {
			return nil, fmt.Errorf("shipment %s arrival was already persisted", value.ID)
		}
		household := string(arrived.ReceiverHouseholdID)
		if arrivals[household] == nil {
			arrivals[household] = map[string]int64{}
		}
		arrivals[household][string(arrived.ResourceType)] += int64(arrived.QuantityMilli)
	}
	return arrivals, nil
}
