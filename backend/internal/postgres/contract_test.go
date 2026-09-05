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
			GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
		}}
		obligations, err := contractObligationsFromRows(rows)
		if err != nil {
			t.Fatalf("travel ticks %d: %v", travelTicks, err)
		}
		expected, err := calendar.LatestDispatchGameDay(140, travelTicks, 91, 12)
		if err != nil {
			t.Fatal(err)
		}
		if obligations[0].LatestDispatchGameDay == nil || *obligations[0].LatestDispatchGameDay != expected {
			t.Fatalf("travel ticks %d latest dispatch = %v, want %d", travelTicks, obligations[0].LatestDispatchGameDay, expected)
		}
	}
}
