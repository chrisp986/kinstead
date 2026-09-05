//go:build postgres

package postgres

import (
	"testing"

	"game/backend/internal/calendar"
	sqlcdb "game/backend/internal/postgres/db"
)

func TestContractObligationsUseSharedDispatchDeadline(t *testing.T) {
	for _, travelTicks := range []int64{1, 2, 5, 8} {
		rows := []sqlcdb.ListContractObligationsRow{{
			ID: "obligation", ContractID: "contract", DebtorHouseholdID: "debtor", CreditorHouseholdID: "creditor",
			ResourceCode: "provisions", QuantityMilli: 1_000, DueArrivalTick: 12, DueGameDay: 140,
			Status: "pending", GameDaySchedule: true, TravelTicks: travelTicks,
			CurrentGameDay: 0, CalendarRemainder: 0,
			GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
		}}
		obligations, err := contractObligationsFromRows(rows)
		if err != nil {
			t.Fatalf("travel ticks %d: %v", travelTicks, err)
		}
		expected, err := calendar.LatestDispatchGameDay(0, 0, 91, 12, 140, travelTicks)
		if err != nil {
			t.Fatal(err)
		}
		if obligations[0].LatestDispatchGameDay == nil || *obligations[0].LatestDispatchGameDay != expected {
			t.Fatalf("travel ticks %d latest dispatch = %v, want %d", travelTicks, obligations[0].LatestDispatchGameDay, expected)
		}
	}

	rows := []sqlcdb.ListContractObligationsRow{{
		ID: "obligation", ContractID: "contract", DebtorHouseholdID: "debtor", CreditorHouseholdID: "creditor",
		ResourceCode: "provisions", QuantityMilli: 1_000, DueArrivalTick: 2, DueGameDay: 22,
		Status: "pending", GameDaySchedule: true, TravelTicks: 2,
		CurrentGameDay: 7, CalendarRemainder: 7,
		GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
	}}
	obligations, err := contractObligationsFromRows(rows)
	if err != nil {
		t.Fatalf("non-zero remainder: %v", err)
	}
	if got := obligations[0].LatestDispatchGameDay; got == nil || *got != 7 {
		t.Fatalf("non-zero remainder latest dispatch = %v, want 7", got)
	}
}
