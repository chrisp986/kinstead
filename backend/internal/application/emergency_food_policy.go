package application

import (
	"sort"

	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

type EmergencyFoodPolicyInput struct {
	State              simulation.HouseholdState
	CurrentTick        int64
	CurrentGameDay     int64
	GameDaysPerTickNum int64
	GameDaysPerTickDen int64
	Season             simulation.Season
	Assignments        []port.AssignmentRecord
	IncomingShipments  []port.ShipmentRecord
	Balance            simulation.BalanceConfig
}

type EmergencyFoodDecision struct {
	Schedule                bool
	CharacterID             string
	Activity                simulation.Activity
	Reason                  string
	SupplyGameDays          int64
	ExpectedProductionMilli int64
	RemainsAtRisk           bool
}

func EvaluateEmergencyFoodPolicy(in EmergencyFoodPolicyInput) EmergencyFoodDecision {
	supplyDays := in.State.SupplyGameDays(in.Balance, in.GameDaysPerTickNum, in.GameDaysPerTickDen)
	decision := EmergencyFoodDecision{SupplyGameDays: supplyDays, RemainsAtRisk: supplyDays < in.Balance.EmergencySupplyDays}
	if supplyDays >= in.Balance.EmergencySupplyDays {
		decision.Reason = "supply_adequate"
		return decision
	}
	// Existing plans may already restore coverage on the next tick. Reuse the
	// simulation so their season, specialization and fatigue effects stay exact.
	planned, err := projectEmergencyNextTick(in, nil)
	if err == nil && planned.FoodShortageMilli == 0 && planned.State.SupplyGameDays(in.Balance, in.GameDaysPerTickNum, in.GameDaysPerTickDen) >= in.Balance.EmergencySupplyDays {
		decision.Reason = "planned_work_and_shipments_cover_supply"
		decision.RemainsAtRisk = false
		return decision
	}
	for _, shipment := range in.IncomingShipments {
		ticksAway := shipment.ExpectedArrivalTick - in.CurrentTick
		if ticksAway < 0 {
			ticksAway = 0
		}
		projected := in.State
		// Arrivals are processed before production and consumption. Only the
		// ticks strictly before arrival must be covered by existing stock.
		if ticksAway > 0 {
			projected.ProvisionsMilli -= (ticksAway - 1) * in.Balance.ConsumptionPerTickMilli
		}
		if projected.ProvisionsMilli < 0 {
			continue
		}
		projected.ProvisionsMilli += shipment.QuantityMilli
		if shipment.ResourceType == "provisions" && shipment.Status == "in_transit" && shipment.QuantityMilli > 0 && projected.SupplyGameDays(in.Balance, in.GameDaysPerTickNum, in.GameDaysPerTickDen) >= in.Balance.EmergencySupplyDays {
			decision.Reason = "incoming_provisions_arrive_in_time"
			decision.RemainsAtRisk = false
			return decision
		}
	}
	nextTick := in.CurrentTick + 1
	blocked := map[string]bool{}
	for _, assignment := range in.Assignments {
		if (assignment.Status == "planned" || assignment.Status == "active") && assignment.StartsTick <= nextTick && assignment.EndsTick >= nextTick {
			blocked[assignment.CharacterID] = true
		}
	}
	type candidate struct {
		character simulation.Character
		activity  simulation.Activity
		output    int64
	}
	candidates := make([]candidate, 0)
	tickContext := simulation.NeutralTickContext(in.Season)
	tickContext.GameDaysPerTickNum, tickContext.GameDaysPerTickDen = in.GameDaysPerTickNum, in.GameDaysPerTickDen
	for _, character := range in.State.Characters {
		if character.LaborPermille <= 0 || character.Fatigue >= 85 || blocked[character.ID] {
			continue
		}
		best := candidate{character: character}
		for _, activity := range []simulation.Activity{simulation.Agriculture, simulation.Fishing} {
			assignment := simulation.Assignment{CharacterID: character.ID, Activity: activity, Intensity: simulation.Normal}
			output := simulation.EstimateProduction(character, assignment, in.State.FarmSpecialization, tickContext, in.Balance)
			if output > best.output {
				best.activity, best.output = activity, output
			}
		}
		if best.output > 0 {
			candidates = append(candidates, best)
		}
	}
	if len(candidates) == 0 {
		decision.Reason = "no_restworthy_free_food_worker"
		return decision
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if (a.character.LaborPermille == 1000) != (b.character.LaborPermille == 1000) {
			return a.character.LaborPermille == 1000
		}
		if a.character.Fatigue != b.character.Fatigue {
			return a.character.Fatigue < b.character.Fatigue
		}
		if a.output != b.output {
			return a.output > b.output
		}
		return a.character.ID < b.character.ID
	})
	selected := candidates[0]
	decision.Schedule = true
	decision.CharacterID = selected.character.ID
	decision.Activity = selected.activity
	decision.ExpectedProductionMilli = selected.output
	decision.Reason = "emergency_food_work_scheduled"
	extra := simulation.Assignment{CharacterID: selected.character.ID, Activity: selected.activity, Intensity: simulation.Normal}
	projected, err := projectEmergencyNextTick(in, &extra)
	decision.RemainsAtRisk = err != nil || projected.FoodShortageMilli > 0 || projected.State.SupplyGameDays(in.Balance, in.GameDaysPerTickNum, in.GameDaysPerTickDen) < in.Balance.EmergencySupplyDays
	return decision
}

func projectEmergencyNextTick(in EmergencyFoodPolicyInput, extra *simulation.Assignment) (simulation.TickResult, error) {
	state := in.State
	state.Tick = in.CurrentTick
	state.Characters = append([]simulation.Character(nil), in.State.Characters...)
	nextTick := in.CurrentTick + 1
	for _, shipment := range in.IncomingShipments {
		if shipment.ResourceType == "provisions" && shipment.Status == "in_transit" && shipment.ExpectedArrivalTick <= nextTick {
			state.ProvisionsMilli += shipment.QuantityMilli
		}
	}
	assignments := []simulation.Assignment{}
	for _, work := range in.Assignments {
		if (work.Status == "planned" || work.Status == "active") && work.StartsTick <= nextTick && work.EndsTick >= nextTick {
			assignments = append(assignments, simulation.Assignment{CharacterID: work.CharacterID, Activity: simulation.Activity(work.Activity), Intensity: simulation.Intensity(work.Intensity)})
		}
	}
	if extra != nil {
		assignments = append(assignments, *extra)
	}
	ctx := simulation.NeutralTickContext(in.Season)
	ctx.GameDaysPerTickNum, ctx.GameDaysPerTickDen = in.GameDaysPerTickNum, in.GameDaysPerTickDen
	return simulation.ProcessTick(state, nextTick, assignments, ctx, in.Balance)
}
