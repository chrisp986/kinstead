package relationship

import (
	"errors"
	"testing"

	contractdomain "game/backend/internal/domain/contract"
	shipmentdomain "game/backend/internal/domain/shipment"
)

func TestStandingForTrustUsesSpecifiedBands(t *testing.T) {
	tests := []struct {
		trust int
		want  Standing
	}{{-100, StandingDisapproving}, {-31, StandingDisapproving}, {-30, StandingNeutral}, {29, StandingNeutral}, {30, StandingFavorable}, {69, StandingFavorable}, {70, StandingConnected}, {100, StandingConnected}}
	for _, tt := range tests {
		got, err := StandingForTrust(tt.trust)
		if err != nil || got != tt.want {
			t.Fatalf("standing(%d) = %s, %v; want %s", tt.trust, got, err, tt.want)
		}
	}
	if _, err := StandingForTrust(101); !errors.Is(err, ErrInvalidTrust) {
		t.Fatalf("out-of-range error = %v", err)
	}
}

func relationshipObligation() contractdomain.Obligation {
	return contractdomain.Obligation{
		ID: "obligation", ContractID: "contract", DebtorHouseholdID: "debtor", CreditorHouseholdID: "creditor",
		ResourceType: "provisions", QuantityMilli: 10_000, DueArrivalTick: 8, ShipmentID: "shipment",
		Status: contractdomain.ObligationDispatched,
	}
}

func TestContractOutcomeEmitsOnlyFinalOutcome(t *testing.T) {
	before := relationshipObligation()
	overdue, err := before.Assess(9, nil)
	if err != nil {
		t.Fatal(err)
	}
	if event, err := ContractOutcome("world", before, overdue, 9); err != nil || event != nil {
		t.Fatalf("overdue event = %+v, %v", event, err)
	}
	arrival := contractdomain.Tick(10)
	late, err := overdue.Assess(10, shipmentTick(10))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ContractOutcome("world", overdue, late, 10)
	if err != nil || event == nil || event.Type != EventContractLate || event.TrustDelta != 0 || event.ActualFulfillmentTick == nil || *event.ActualFulfillmentTick != arrival {
		t.Fatalf("late event = %+v, %v", event, err)
	}
	broken, err := before.Assess(11, nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err = ContractOutcome("world", before, broken, 11)
	if err != nil || event == nil || event.Type != EventContractBroken || event.SourceHouseholdID != "creditor" || event.TargetHouseholdID != "debtor" {
		t.Fatalf("broken event = %+v, %v", event, err)
	}
}

func shipmentTick(value int64) *shipmentdomain.Tick {
	tick := shipmentdomain.Tick(value)
	return &tick
}
