package simulation_test

import (
	"reflect"
	"testing"

	"game/backend/internal/balance"
	"game/backend/internal/scenario/v03"
	"game/backend/internal/simulation"
)

func TestProcessTickUsesCallerCalendarContext(t *testing.T) {
	state := v03.NewBjornvikState()
	assignment := []simulation.Assignment{{CharacterID: "bjorn", Activity: simulation.Agriculture, Intensity: simulation.Normal}}
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

func TestProcessTickUsesCharacterIDsWhenNamesAreDuplicated(t *testing.T) {
	cfg := balance.V03()
	state := simulation.HouseholdState{ProvisionsMilli: 100_000, Characters: []simulation.Character{
		{ID: "first", Name: "Einar", LaborPermille: 1000, Specialization: simulation.Agriculture},
		{ID: "second", Name: "Einar", LaborPermille: 1000, Specialization: simulation.Fishing},
	}}
	result, err := simulation.ProcessTick(state, 1, []simulation.Assignment{
		{CharacterID: "first", Activity: simulation.Agriculture, Intensity: simulation.Normal},
		{CharacterID: "second", Activity: simulation.Fishing, Intensity: simulation.Normal},
	}, simulation.NeutralTickContext(simulation.Spring), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProducedProvisionsMilli != 6325 {
		t.Fatalf("production = %d, want both same-named characters' output 6325", result.ProducedProvisionsMilli)
	}
}

func TestProcessTickRejectsDuplicateAndUnknownCharacterIDsAtomically(t *testing.T) {
	cfg := balance.V03()
	base := simulation.HouseholdState{ProvisionsMilli: 100_000, Characters: []simulation.Character{{ID: "worker", Name: "Einar", LaborPermille: 1000}}}
	for name, assignments := range map[string][]simulation.Assignment{
		"duplicate": {
			{CharacterID: "worker", Activity: simulation.Agriculture, Intensity: simulation.Normal},
			{CharacterID: "worker", Activity: simulation.Fishing, Intensity: simulation.Normal},
		},
		"unknown": {{CharacterID: "missing", Activity: simulation.Fishing, Intensity: simulation.Normal}},
	} {
		t.Run(name, func(t *testing.T) {
			before := base
			before.Characters = append([]simulation.Character(nil), base.Characters...)
			state := before
			state.Characters = append([]simulation.Character(nil), before.Characters...)
			if _, err := simulation.ProcessTick(state, 1, assignments, simulation.NeutralTickContext(simulation.Spring), cfg); err == nil {
				t.Fatal("expected validation error")
			}
			if !reflect.DeepEqual(state, before) {
				t.Fatalf("input state mutated on failure: %#v", state)
			}
		})
	}
}

func TestSupplyCoverageUsesHistoricalPacingAndNotWallClockDuration(t *testing.T) {
	cfg := balance.V03()
	state := simulation.HouseholdState{ProvisionsMilli: 4 * cfg.ConsumptionPerTickMilli}
	if got := state.SupplyTicks(cfg); got != 4 {
		t.Fatalf("supply ticks = %d, want 4", got)
	}
	if got := state.SupplyGameDays(cfg, 91, 12); got != 30 {
		t.Fatalf("supply game days = %d, want 30", got)
	}
	// Wall-clock tick duration is intentionally not an input to either result.
	for _, wallClockSeconds := range []int64{60, 3600, 21600} {
		_ = wallClockSeconds
		if got := state.SupplyGameDays(cfg, 91, 12); got != 30 {
			t.Fatalf("coverage changed with execution cadence: %d", got)
		}
	}
}

func TestSupplyCoverageFloorsAfterConvertingFractionalTicks(t *testing.T) {
	cfg := balance.V03()
	state := simulation.HouseholdState{ProvisionsMilli: cfg.ConsumptionPerTickMilli - 1}
	if got := state.SupplyTicks(cfg); got != 0 {
		t.Fatalf("supply ticks = %d, want 0 whole ticks", got)
	}
	if got := state.SupplyGameDays(cfg, 91, 12); got != 7 {
		t.Fatalf("supply game days = %d, want 7", got)
	}
	if state.SupplyBelowGameDays(cfg, 7, 91, 12) {
		t.Fatal("nearly one full tick of provisions covers more than seven game days")
	}
}

func TestSupplyStatusBoundariesUseGameDays(t *testing.T) {
	cfg := balance.V03()
	for _, tc := range []struct {
		name string
		days int64
		want string
	}{
		{"emergency", 6, "emergency"}, {"critical", 7, "critical"}, {"critical-high", 14, "critical"},
		{"strained-low", 15, "strained"}, {"strained-high", 30, "strained"}, {"safe", 31, "safe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := simulation.SupplyStatus(tc.days, cfg)
			if got != tc.want {
				t.Fatalf("status(%d) = %q, want %q", tc.days, got, tc.want)
			}
		})
	}
}

func TestProcessTickClampsAndReportsFoodShortage(t *testing.T) {
	cfg := balance.V03()
	state := simulation.HouseholdState{ProvisionsMilli: 1000, Characters: []simulation.Character{}}
	result, err := simulation.ProcessTick(state, 1, nil, simulation.NeutralTickContext(simulation.Winter), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.State.ProvisionsMilli != 0 || result.FoodShortageMilli != 3900 {
		t.Fatalf("provisions/shortage=%d/%d", result.State.ProvisionsMilli, result.FoodShortageMilli)
	}
}

func TestSupplyCoveragePreservesIntegersBeyondFloatPrecision(t *testing.T) {
	cfg := balance.V03()
	state := simulation.HouseholdState{ProvisionsMilli: 9223372036854775807}
	if got := state.SupplyGameDays(cfg, 91, 12); got != 14274266247513343 {
		t.Fatalf("overflow-safe coverage=%d", got)
	}
	cfg.ConsumptionPerTickMilli = 1
	state.ProvisionsMilli = 9007199254740993
	if got := state.SupplyGameDays(cfg, 1, 1); got != 9007199254740993 {
		t.Fatalf("integer precision lost: %d", got)
	}
}
