package simulation_test

import (
	"testing"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/simulation"
)

func dailyTestState() simulation.DailyLaborState {
	return simulation.DailyLaborState{
		CurrentGameDay: 0, ProvisionsMilli: 10_000, WoodMilli: 100_000,
		Characters: []simulation.DailyCharacter{{ID: "astrid", BirthGameDay: -20 * 365, LaborPermille: 1000, Status: "active", Occupation: workdomain.Occupation{CharacterID: "astrid", Activity: workdomain.Fishing, Revision: 1}}},
	}
}

func runDailyHours(t *testing.T, state simulation.DailyLaborState, from, count int) simulation.DailyLaborState {
	t.Helper()
	cfg := balance.DailyLaborV1()
	for h := 0; h < count; h++ {
		start := calendar.Moment{Day: 0, Hour: from + h}
		if start.Hour >= 24 {
			start = calendar.AdvanceMoment(start, 0)
		}
		result, err := simulation.ProcessHour(state, simulation.HourInterval{Start: start, End: calendar.AdvanceMoment(start, 1)}, cfg)
		if err != nil {
			t.Fatalf("hour %d: %v", h, err)
		}
		state = result.State
	}
	return state
}

func TestDailyLaborWorkdaySettlesAtSeventeen(t *testing.T) {
	cfg := balance.DailyLaborV1()
	state := dailyTestState()
	state = runDailyHours(t, state, 8, 8)
	if state.ProvisionsMilli != 10_000-200 || state.PendingProvisionsMilli == 0 {
		t.Fatalf("before settlement stocks=%d pending=%d", state.ProvisionsMilli, state.PendingProvisionsMilli)
	}
	result, err := simulation.ProcessHour(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: 16}, End: calendar.Moment{Day: 0, Hour: 17}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Settled || result.State.PendingProvisionsMilli != 0 || result.State.ProvisionsMilli <= 10_000 {
		t.Fatalf("settlement result=%+v", result)
	}
}

func TestDailyLaborHonorsPartialWorkAndRecovery(t *testing.T) {
	state := dailyTestState()
	state.Characters[0].LaborPermille = 500
	state = runDailyHours(t, state, 8, 3)
	if state.PendingProvisionsMilli <= 0 || state.PendingProvisionsMilli >= 6_500 {
		t.Fatalf("partial output=%d", state.PendingProvisionsMilli)
	}
	state.Characters[0].Fatigue = 20
	result, err := simulation.ProcessHour(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: 17}, End: calendar.Moment{Day: 0, Hour: 18}}, balance.DailyLaborV1())
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Characters[0].Fatigue != 18 {
		t.Fatalf("recovery fatigue=%d", result.State.Characters[0].Fatigue)
	}
}

func TestDailyLaborOccupationChangeAtNextWorkday(t *testing.T) {
	state := dailyTestState()
	pending := workdomain.Agriculture
	state.Characters[0].Occupation.PendingActivity = &pending
	day := calendar.GameDay(1)
	state.Characters[0].Occupation.EffectiveDay = &day
	result, err := simulation.ProcessHour(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: 8}, End: calendar.Moment{Day: 0, Hour: 9}}, balance.DailyLaborV1())
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Characters[0].Occupation.Activity != workdomain.Fishing {
		t.Fatal("pending occupation became active early")
	}
	result, err = simulation.ProcessHour(result.State, simulation.HourInterval{Start: calendar.Moment{Day: 1, Hour: 8}, End: calendar.Moment{Day: 1, Hour: 9}}, balance.DailyLaborV1())
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Characters[0].Occupation.Activity != workdomain.Agriculture || result.State.Characters[0].Occupation.PendingActivity != nil {
		t.Fatal("pending occupation did not resolve at 08:00")
	}
}

func TestDailyLaborServiceSuspendsHomeWork(t *testing.T) {
	state := dailyTestState()
	state.Characters[0].TemporaryDuties = []workdomain.TemporaryDuty{{ID: "duty", Activity: workdomain.RulerService, Starts: calendar.Moment{Day: 0, Hour: 8}, Ends: calendar.Moment{Day: 0, Hour: 10}}}
	result, err := simulation.ProcessHour(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: 8}, End: calendar.Moment{Day: 0, Hour: 9}}, balance.DailyLaborV1())
	if err != nil {
		t.Fatal(err)
	}
	if result.ProducedProvisionsMilli != 0 {
		t.Fatalf("service produced home output=%d", result.ProducedProvisionsMilli)
	}
	result, err = simulation.ProcessHour(result.State, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: 10}, End: calendar.Moment{Day: 0, Hour: 11}}, balance.DailyLaborV1())
	if err != nil {
		t.Fatal(err)
	}
	if result.ProducedProvisionsMilli == 0 {
		t.Fatal("home occupation did not resume")
	}
}
