package simulation_test

import (
	"testing"

	"game/backend/internal/balance"
	"game/backend/internal/simulation"
)

func TestRulerServiceCreatesProductionOpportunityCost(t *testing.T) {
	cfg := balance.V03()
	state := simulation.HouseholdState{FarmSpecialization: simulation.Agriculture, ProvisionsMilli: 1_000_000, Tick: 0, Characters: []simulation.Character{
		{Name: "Einar", LaborPermille: 1000, Specialization: simulation.Agriculture},
		{Name: "Astrid", LaborPermille: 1000, Specialization: simulation.Fishing},
	}}
	ctx := simulation.NeutralTickContext(simulation.Spring)
	productiveState := state
	productiveState.Characters = append([]simulation.Character(nil), state.Characters...)
	productive, err := simulation.ProcessTick(productiveState, 1, []simulation.Assignment{{Character: "Einar", Activity: simulation.Agriculture, Intensity: simulation.Normal}, {Character: "Astrid", Activity: simulation.Fishing, Intensity: simulation.Normal}}, ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	serviceState := state
	for tick := int64(1); tick <= 4; tick++ {
		service := simulation.Assignment{Character: "Einar", Activity: simulation.RulerService, Intensity: simulation.Normal}
		astridWork := simulation.Assignment{Character: "Astrid", Activity: simulation.Fishing, Intensity: simulation.Normal}
		if production := simulation.EstimateProduction(serviceState.Characters[0], service, serviceState.FarmSpecialization, ctx, cfg); production != 0 {
			t.Fatalf("ruler service produced %d; it must produce no resources", production)
		}
		astridProduction := simulation.EstimateProduction(serviceState.Characters[1], astridWork, serviceState.FarmSpecialization, ctx, cfg)
		result, err := simulation.ProcessTick(serviceState, tick, []simulation.Assignment{service, astridWork}, ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if result.ProducedProvisionsMilli != astridProduction || result.ProducedWoodMilli != 0 {
			t.Fatalf("service tick production food/wood=%d/%d, want only Astrid's %d/0", result.ProducedProvisionsMilli, result.ProducedWoodMilli, astridProduction)
		}
		if tick == 1 && result.ProducedProvisionsMilli >= productive.ProducedProvisionsMilli {
			t.Fatalf("service production %d must be below productive output %d", result.ProducedProvisionsMilli, productive.ProducedProvisionsMilli)
		}
		serviceState = result.State
	}
	if serviceState.Characters[0].Fatigue != 16 {
		t.Fatalf("Einar fatigue = %d, want 16 after four normal service ticks", serviceState.Characters[0].Fatigue)
	}
	if serviceState.Characters[1].Fatigue != 16 {
		t.Fatalf("Astrid should continue normal work, fatigue = %d", serviceState.Characters[1].Fatigue)
	}
	returned, err := simulation.ProcessTick(serviceState, 5, []simulation.Assignment{{Character: "Einar", Activity: simulation.Agriculture, Intensity: simulation.Normal}, {Character: "Astrid", Activity: simulation.Fishing, Intensity: simulation.Normal}}, ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if returned.ProducedProvisionsMilli <= 0 {
		t.Fatal("productive output did not return after service")
	}
}
