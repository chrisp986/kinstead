package application

import (
	"testing"

	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

func TestBuildTickDiagnosticVerifiesLegacyStoredAccounting(t *testing.T) {
	world := port.WorldClaim{ID: "world", SimulationModel: port.ModelLegacy}
	opening := port.HouseholdAccountingState{
		StoredMilli:        map[string]int64{"provisions": 10_000, "wood": 2_000},
		PendingOutputMilli: map[string]int64{},
	}
	closing := port.HouseholdAccountingState{
		StoredMilli:        map[string]int64{"provisions": 12_000, "wood": 1_500},
		PendingOutputMilli: map[string]int64{},
	}
	value, err := buildTickDiagnostic(world, "household", 1, 0, 1, opening, closing, simulation.TickResult{
		ProducedProvisionsMilli: 2_000,
		ProducedWoodMilli:       500,
		FoodShortageMilli:       1_000,
		WoodShortageMilli:       0,
	}, simulation.HourResult{}, simulation.BalanceConfig{ConsumptionPerTickMilli: 1_000, DailyWoodUpkeepMilli: 1_000}, map[string]int64{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Details["tracked_balance_check"] != "passed" {
		t.Fatalf("details=%v", value.Details)
	}
	actual := value.Details["actual_consumption_milli"].(map[string]int64)
	if actual["provisions"] != 0 || actual["wood"] != 1_000 {
		t.Fatalf("actual consumption=%v", actual)
	}
}

func TestBuildTickDiagnosticRejectsUnexplainedStoredDifference(t *testing.T) {
	world := port.WorldClaim{ID: "world", SimulationModel: port.ModelLegacy}
	opening := port.HouseholdAccountingState{StoredMilli: map[string]int64{"provisions": 10_000}, PendingOutputMilli: map[string]int64{}}
	closing := port.HouseholdAccountingState{StoredMilli: map[string]int64{"provisions": 9_999}, PendingOutputMilli: map[string]int64{}}
	_, err := buildTickDiagnostic(world, "household", 1, 0, 1, opening, closing, simulation.TickResult{}, simulation.HourResult{}, simulation.BalanceConfig{}, nil)
	if err == nil {
		t.Fatal("expected accounting mismatch")
	}
}
