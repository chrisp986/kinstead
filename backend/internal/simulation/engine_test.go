package simulation_test

import (
	"testing"

	"game/backend/internal/balance"
	"game/backend/internal/scenario/v03"
	"game/backend/internal/simulation"
)

func TestProcessTickUsesCallerCalendarContext(t *testing.T) {
	state := v03.NewBjornvikState()
	assignment := []simulation.Assignment{{Character: "Bjorn", Activity: simulation.Agriculture, Intensity: simulation.Normal}}
	cfg := balance.V03()

	spring, err := simulation.ProcessTick(state, 1, assignment, simulation.NeutralTickContext(simulation.Spring), cfg)
	if err != nil {
		t.Fatal(err)
	}
	winter, err := simulation.ProcessTick(state, 1, assignment, simulation.NeutralTickContext(simulation.Winter), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if spring.ProducedProvisionsMilli != 2587 || winter.ProducedProvisionsMilli != 1035 {
		t.Fatalf("spring/winter production = %d/%d", spring.ProducedProvisionsMilli, winter.ProducedProvisionsMilli)
	}
}
