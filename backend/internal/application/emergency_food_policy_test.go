package application

import (
	"testing"

	"game/backend/internal/balance"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

func emergencyInput() EmergencyFoodPolicyInput {
	cfg := balance.V03()
	return EmergencyFoodPolicyInput{
		State: simulation.HouseholdState{ProvisionsMilli: cfg.ConsumptionPerTickMilli / 2, FarmSpecialization: simulation.Fishing, Characters: []simulation.Character{
			{ID: "tired", Name: "Same", LaborPermille: 1000, Fatigue: 60, Specialization: simulation.Agriculture},
			{ID: "ready", Name: "Same", LaborPermille: 1000, Fatigue: 5, Specialization: simulation.Fishing},
		}},
		CurrentTick: 4, CurrentGameDay: 30, GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
		Season: simulation.Spring, Balance: cfg,
	}
}

func TestEmergencyFoodPolicyCreatesNormalSuitableAssignment(t *testing.T) {
	decision := EvaluateEmergencyFoodPolicy(emergencyInput())
	if !decision.Schedule || decision.CharacterID != "ready" || decision.Activity != simulation.Fishing || decision.ExpectedProductionMilli <= 0 {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestEmergencyFoodPolicyIncomingShipmentProtectsHousehold(t *testing.T) {
	in := emergencyInput()
	in.IncomingShipments = []port.ShipmentRecord{{ResourceType: "provisions", QuantityMilli: 10_000, ExpectedArrivalGameDay: 30, Status: "in_transit"}}
	decision := EvaluateEmergencyFoodPolicy(in)
	if decision.Schedule || decision.Reason != "planned_work_and_shipments_cover_supply" || decision.RemainsAtRisk {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestEmergencyFoodPolicyAccountsForPlannedProduction(t *testing.T) {
	in := emergencyInput()
	in.Season = simulation.Summer
	in.State.ProvisionsMilli = 4400
	in.Assignments = []port.AssignmentRecord{{CharacterID: "ready", Activity: "fishing", Intensity: "normal", StartsTick: 5, EndsTick: 5, Status: "planned"}}
	decision := EvaluateEmergencyFoodPolicy(in)
	if decision.Schedule || decision.RemainsAtRisk || decision.Reason != "planned_work_and_shipments_cover_supply" {
		t.Fatalf("decision=%+v", decision)
	}
	if in.State.Characters[1].Fatigue != 5 {
		t.Fatal("policy mutated authoritative worker state")
	}
}

func TestEmergencyFoodPolicyArrivalPrecedesConsumption(t *testing.T) {
	for _, tc := range []struct {
		name         string
		arrivalTick  int64
		wantSchedule bool
	}{
		{"next tick prevents shortage", 5, false},
		{"following tick is too late", 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := emergencyInput()
			in.IncomingShipments = []port.ShipmentRecord{{ResourceType: "provisions", QuantityMilli: 10_000, ExpectedArrivalTick: tc.arrivalTick, Status: "in_transit"}}
			if got := EvaluateEmergencyFoodPolicy(in); got.Schedule != tc.wantSchedule {
				t.Fatalf("decision = %+v", got)
			}
		})
	}
}

func TestEmergencyFoodPolicyRespectsPlansAndExhaustion(t *testing.T) {
	in := emergencyInput()
	in.State.Characters[0].Fatigue, in.State.Characters[1].Fatigue = 90, 90
	if decision := EvaluateEmergencyFoodPolicy(in); decision.Schedule || decision.Reason != "no_restworthy_free_food_worker" || !decision.RemainsAtRisk {
		t.Fatalf("exhausted decision = %+v", decision)
	}
	in = emergencyInput()
	in.Assignments = []port.AssignmentRecord{{ID: "plan-1", CharacterID: "ready", StartsTick: 5, EndsTick: 6, Status: "planned"}, {ID: "plan-2", CharacterID: "tired", StartsTick: 5, EndsTick: 5, Status: "planned"}}
	if decision := EvaluateEmergencyFoodPolicy(in); decision.Schedule || decision.Reason != "no_restworthy_free_food_worker" {
		t.Fatalf("planned decision = %+v", decision)
	}
}

func TestEmergencyFoodPolicyAvoidsDuplicateEmergencyAssignment(t *testing.T) {
	in := emergencyInput()
	in.Assignments = []port.AssignmentRecord{{ID: "emergency", CharacterID: "ready", StartsTick: 5, EndsTick: 5, Status: "planned"}}
	decision := EvaluateEmergencyFoodPolicy(in)
	if !decision.Schedule || decision.CharacterID != "tired" {
		t.Fatalf("decision = %+v", decision)
	}
	// With every worker already covered, the policy does not create another assignment.
	in.Assignments = append(in.Assignments, port.AssignmentRecord{ID: "other", CharacterID: "tired", StartsTick: 5, EndsTick: 5, Status: "active"})
	if decision = EvaluateEmergencyFoodPolicy(in); decision.Schedule {
		t.Fatalf("duplicate decision = %+v", decision)
	}
}

func TestEmergencyFoodPolicyWinterShortageReportsRemainingRisk(t *testing.T) {
	in := emergencyInput()
	in.Season = simulation.Winter
	decision := EvaluateEmergencyFoodPolicy(in)
	if !decision.Schedule || !decision.RemainsAtRisk || decision.ExpectedProductionMilli <= 0 {
		t.Fatalf("winter decision = %+v", decision)
	}
}

func TestEmergencyFoodPolicyUnattendedAcrossSeasons(t *testing.T) {
	for _, season := range []simulation.Season{simulation.Spring, simulation.Summer, simulation.Autumn, simulation.Winter} {
		in := emergencyInput()
		in.Season = season
		decision := EvaluateEmergencyFoodPolicy(in)
		if !decision.Schedule || decision.CharacterID == "" {
			t.Fatalf("%s decision = %+v", season, decision)
		}
	}
}
