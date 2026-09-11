package simulation_test

import (
	"testing"
	"time"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/simulation"
)

func monthlyContext(t *testing.T, date time.Time) simulation.WorkContext {
	t.Helper()
	position, err := calendar.SeasonalPositionForDate(date)
	if err != nil {
		t.Fatal(err)
	}
	workday, err := calendar.WorkdayForDate(date, calendar.DefaultDaylightConfig())
	if err != nil {
		t.Fatal(err)
	}
	return simulation.WorkContext{Season: position.Season, Workday: workday}
}

func TestMonthlySettlementUsesFullPendingTotalsBeforeConsumption(t *testing.T) {
	state := dailyTestState()
	state.ProvisionsMilli = 0
	state.PendingProvisionsMilli = 500
	ctx := monthlyContext(t, time.Date(2028, time.April, 1, 0, 0, 0, 0, time.UTC))
	result, err := simulation.ProcessHourWithContext(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: 14}, End: calendar.Moment{Day: 0, Hour: 15}}, ctx, balance.MonthlySeasonsV1())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Settled || result.SettledProvisionsMilli != 500+result.ProducedProvisionsMilli || result.State.PendingProvisionsMilli != 0 || result.State.ProvisionsMilli != result.SettledProvisionsMilli-result.ConsumedProvisionsMilli {
		t.Fatalf("settlement=%+v", result)
	}
}

func TestMonthlyWinterShortDayEarnsHourlyShares(t *testing.T) {
	state := dailyTestState()
	ctx := monthlyContext(t, time.Date(2028, time.April, 1, 0, 0, 0, 0, time.UTC))
	var earned int64
	for hour := ctx.Workday.StartHour; hour < ctx.Workday.EndHour; hour++ {
		result, err := simulation.ProcessHourWithContext(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: hour}, End: calendar.Moment{Day: 0, Hour: hour + 1}}, ctx, balance.MonthlySeasonsV1())
		if err != nil {
			t.Fatal(err)
		}
		earned += result.ProducedProvisionsMilli
		state = result.State
	}
	if ctx.Workday.EndHour-ctx.Workday.StartHour != 5 || earned <= 0 || earned >= 8500 {
		t.Fatalf("workday=%+v earned=%d", ctx.Workday, earned)
	}
}

func TestMonthlyModelPreservesSelectedOccupationAtLowFood(t *testing.T) {
	state := dailyTestState()
	state.ProvisionsMilli = 0
	state.Characters[0].Occupation.Activity = workdomain.Woodcutting
	ctx := monthlyContext(t, time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC))
	result, err := simulation.ProcessHourWithContext(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: ctx.Workday.StartHour}, End: calendar.Moment{Day: 0, Hour: ctx.Workday.StartHour + 1}}, ctx, balance.MonthlySeasonsV1())
	if err != nil {
		t.Fatal(err)
	}
	if result.ProducedWoodMilli == 0 || result.ProducedProvisionsMilli != 0 || result.State.Characters[0].Occupation.Activity != workdomain.Woodcutting {
		t.Fatalf("result=%+v", result)
	}
}

func TestMonthlyOccupationChoicesProduceDifferentResources(t *testing.T) {
	ctx := monthlyContext(t, time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC))
	for _, tt := range []struct {
		activity workdomain.Activity
		food     bool
	}{{workdomain.Agriculture, true}, {workdomain.Fishing, true}, {workdomain.Woodcutting, false}} {
		state := dailyTestState()
		state.Characters[0].Occupation.Activity = tt.activity
		result, err := simulation.ProcessHourWithContext(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: ctx.Workday.StartHour}, End: calendar.Moment{Day: 0, Hour: ctx.Workday.StartHour + 1}}, ctx, balance.MonthlySeasonsV1())
		if err != nil {
			t.Fatal(err)
		}
		if tt.food && (result.ProducedProvisionsMilli == 0 || result.ProducedWoodMilli != 0) {
			t.Fatalf("%s produced food=%d wood=%d", tt.activity, result.ProducedProvisionsMilli, result.ProducedWoodMilli)
		}
		if !tt.food && (result.ProducedWoodMilli == 0 || result.ProducedProvisionsMilli != 0) {
			t.Fatalf("%s produced food=%d wood=%d", tt.activity, result.ProducedProvisionsMilli, result.ProducedWoodMilli)
		}
	}
}

func TestMonthlyOccupationAppliesAtCalculatedHour(t *testing.T) {
	state := dailyTestState()
	pending := workdomain.Agriculture
	day, hour := calendar.GameDay(0), 9
	state.Characters[0].Occupation.PendingActivity = &pending
	state.Characters[0].Occupation.EffectiveDay = &day
	state.Characters[0].Occupation.EffectiveHour = &hour
	ctx := monthlyContext(t, time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC))
	result, err := simulation.ProcessHourWithContext(state, simulation.HourInterval{Start: calendar.Moment{Day: 0, Hour: 9}, End: calendar.Moment{Day: 0, Hour: 10}}, ctx, balance.MonthlySeasonsV1())
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Characters[0].Occupation.Activity != pending || result.State.Characters[0].Occupation.PendingActivity != nil {
		t.Fatalf("occupation=%+v", result.State.Characters[0].Occupation)
	}
}
