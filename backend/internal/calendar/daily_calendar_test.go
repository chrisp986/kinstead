package calendar

import "testing"

func TestDailyLaborCalendarUses365DayBoundaries(t *testing.T) {
	if got := DailyLaborDefinition.ProductionSeasonAt(182); got != Summer {
		t.Fatalf("day 182 season=%s", got)
	}
	if got := DailyLaborDefinition.ProductionSeasonAt(183); got != Autumn {
		t.Fatalf("day 183 season=%s", got)
	}
	if got := DailyLaborDefinition.ProductionSeasonAt(273); got != Autumn {
		t.Fatalf("day 273 season=%s", got)
	}
	if got := DailyLaborDefinition.ProductionSeasonAt(274); got != Winter {
		t.Fatalf("day 274 season=%s", got)
	}
	if got := DailyLaborDefinition.StartOfNextHalfYear(182); got != 183 {
		t.Fatalf("next half=%d", got)
	}
	if got := DailyLaborDefinition.StartOfNextProductionSeason(364); got != 365 {
		t.Fatalf("next season=%d", got)
	}
	if got, err := DailyLaborDefinition.Age(-365*10-10, 0); err != nil || got != 10 {
		t.Fatalf("age=%d err=%v", got, err)
	}
	if DailyLaborDefinition.Breakdown(364).DayOfWeek != 1 || DailyLaborDefinition.Breakdown(365).DayOfWeek != 2 {
		t.Fatalf("daily weekdays did not continue across year boundary")
	}
}

func TestDailyLaborClockAndSettlementBoundaries(t *testing.T) {
	clock := ClockState{Day: 4, Remainder: 7, GameDaysPerTickNum: 1, GameDaysPerTickDen: 24}
	moment, err := MomentAtClock(clock)
	if err != nil || moment != (Moment{Day: 4, Hour: 7}) {
		t.Fatalf("moment=%+v err=%v", moment, err)
	}
	if IsWorkingHour(7) || !IsWorkingHour(8) || !IsWorkingHour(16) || IsWorkingHour(17) {
		t.Fatal("working-hour boundary incorrect")
	}
	if !CrossesSettlement(Moment{Day: 4, Hour: 16}, Moment{Day: 4, Hour: 17}) {
		t.Fatal("16:00 interval did not settle")
	}
	if _, err := NextWorkStart(clock); err != nil {
		t.Fatal(err)
	}
}
