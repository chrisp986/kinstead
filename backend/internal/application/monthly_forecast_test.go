package application

import (
	"testing"
	"time"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

func monthlyForecastSnapshot() port.HouseholdSnapshot {
	anchor := time.Date(2028, time.January, 31, 0, 0, 0, 0, time.UTC)
	offset := 0
	state := simulation.DailyLaborState{Tick: 0, CurrentGameDay: 0, CalendarRemainder: 23, ProvisionsMilli: 10_000, WoodMilli: 100_000,
		Characters: []simulation.DailyCharacter{{ID: "worker", BirthGameDay: -20 * 365, LaborPermille: 1000, Status: "active", Occupation: workdomain.Occupation{CharacterID: "worker", Activity: workdomain.Fishing, Revision: 1}}}}
	return port.HouseholdSnapshot{SimulationModel: port.ModelMonthlySeasons, CurrentTick: 0, CurrentGameDay: 0, CalendarRemainder: 23, GameDaysPerTickNum: 1, GameDaysPerTickDen: 24, CalendarAnchorAt: &anchor, WorldUTCOffsetMinutes: &offset, DailyLabor: &state}
}

func TestMonthlyForecastCrossesSeasonAndIncludesConfirmedShipment(t *testing.T) {
	withShipment := monthlyForecastSnapshot()
	withShipment.IncomingShipments = []port.ShipmentRecord{{ID: "delivery", ResourceType: "provisions", QuantityMilli: 4_000, ExpectedArrivalTick: 1, Status: "in_transit"}}
	with, err := ForecastHousehold(withShipment, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	withoutSnapshot := monthlyForecastSnapshot()
	without, err := ForecastHousehold(withoutSnapshot, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if difference := with.BaselineEndingStocks.ProvisionsMilli - without.BaselineEndingStocks.ProvisionsMilli; difference != 4_000 {
		t.Fatalf("shipment difference=%d with=%+v without=%+v", difference, with.BaselineEndingStocks, without.BaselineEndingStocks)
	}
	// The horizon begins at Jan 31 23:00 and works during Feb 1 (summer).
	if with.BaselineEndingStocks.ProvisionsMilli <= 14_000 {
		t.Fatalf("summer production was not applied: %+v", with.BaselineEndingStocks)
	}
	if with.SnapshotTick != 0 || len(with.Assumptions) == 0 {
		t.Fatalf("forecast metadata=%+v", with)
	}

	// Replay the same copied state with execution's interval ordering. This
	// catches forecast-only calendar or arrival rules drifting from the worker.
	executed := cloneDailyState(*withShipment.DailyLabor)
	start := calendar.Moment{Day: 0, Hour: 23}
	for elapsed := 0; elapsed < 24; elapsed++ {
		begin := calendar.AdvanceMoment(start, elapsed)
		if executed.Tick+1 == withShipment.IncomingShipments[0].ExpectedArrivalTick {
			executed.ProvisionsMilli += withShipment.IncomingShipments[0].QuantityMilli
		}
		temporal, err := temporalContext(withShipment.SimulationModel, withShipment.CalendarAnchorAt, withShipment.WorldUTCOffsetMinutes, begin)
		if err != nil {
			t.Fatal(err)
		}
		result, err := simulation.ProcessHourWithContext(executed, simulation.HourInterval{Start: begin, End: calendar.AdvanceMoment(begin, 1)}, temporal.Context, balance.MonthlySeasonsV1())
		if err != nil {
			t.Fatal(err)
		}
		executed = result.State
	}
	if with.BaselineEndingStocks.ProvisionsMilli != executed.ProvisionsMilli || with.BaselineEndingStocks.WoodMilli != executed.WoodMilli || with.BaselineEndingStocks.PendingProvisionsMilli != executed.PendingProvisionsMilli {
		t.Fatalf("forecast=%+v execution=%+v", with.BaselineEndingStocks, executed)
	}
}

func TestShiftMomentKeepsCommitmentPastStart(t *testing.T) {
	got, err := calendar.ShiftMoment(calendar.Moment{Day: 4, Hour: 2}, -5)
	if err != nil {
		t.Fatal(err)
	}
	if got != (calendar.Moment{Day: 3, Hour: 21}) {
		t.Fatalf("shift=%+v", got)
	}
}
