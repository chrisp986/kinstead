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

func TestTrustDeltaForContractOutcome(t *testing.T) {
	due := contractdomain.Tick(10)
	arrival := func(value int64) *contractdomain.Tick {
		tick := contractdomain.Tick(value)
		return &tick
	}
	tests := []struct {
		name      string
		eventType EventType
		fulfilled *contractdomain.Tick
		wantDelta int
	}{
		{"fulfilled on time", EventContractFulfilled, arrival(10), TrustDeltaContractFulfilled},
		{"fulfilled early", EventContractFulfilled, arrival(9), TrustDeltaContractFulfilled},
		{"late one tick", EventContractLate, arrival(11), TrustDeltaContractLateOneTick},
		{"late two ticks", EventContractLate, arrival(12), TrustDeltaContractLateTwoTicks},
		{"broken unresolved", EventContractBroken, nil, TrustDeltaContractBroken},
		{"broken eventual arrival", EventContractBroken, arrival(13), TrustDeltaContractBroken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TrustDeltaForContractOutcome(tt.eventType, due, tt.fulfilled)
			if err != nil || got != tt.wantDelta {
				t.Fatalf("trust delta = %d, %v; want %d", got, err, tt.wantDelta)
			}
		})
	}
}

func TestTrustDeltaRejectsInvalidOutcomeCombinations(t *testing.T) {
	due := contractdomain.Tick(10)
	invalid := []struct {
		name      string
		eventType EventType
		fulfilled *contractdomain.Tick
	}{
		{"late without fulfillment", EventContractLate, nil},
		{"late on due tick", EventContractLate, contractTick(10)},
		{"late three ticks", EventContractLate, contractTick(13)},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := TrustDeltaForContractOutcome(tt.eventType, due, tt.fulfilled); !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidEvent)
			}
		})
	}
}

func TestTrustDeltaForContractOutcomeGameDayUsesCalendarBuckets(t *testing.T) {
	due := contractdomain.GameDay(100)
	for _, test := range []struct {
		name    string
		arrival contractdomain.GameDay
		want    int
	}{
		{"on time", 100, TrustDeltaContractFulfilled},
		{"late tier one", 107, TrustDeltaContractLateOneTick},
		{"late tier two", 114, TrustDeltaContractLateTwoTicks},
	} {
		t.Run(test.name, func(t *testing.T) {
			arrival := test.arrival
			typeForOutcome := EventContractFulfilled
			if arrival > due && arrival <= due+14 {
				typeForOutcome = EventContractLate
			}
			got, err := TrustDeltaForContractOutcomeGameDay(typeForOutcome, due, &arrival)
			if err != nil || got != test.want {
				t.Fatalf("trust delta = %d, %v; want %d", got, err, test.want)
			}
		})
	}
	arrival := contractdomain.GameDay(115)
	if got, err := TrustDeltaForContractOutcomeGameDay(EventContractBroken, due, &arrival); err != nil || got != TrustDeltaContractBroken {
		t.Fatalf("broken trust delta = %d, %v; want %d", got, err, TrustDeltaContractBroken)
	}
}

func TestContractOutcomeEmitsFinalTrustConsequences(t *testing.T) {
	tests := []struct {
		name      string
		current   contractdomain.Tick
		arrival   *shipmentdomain.Tick
		wantType  EventType
		wantDelta int
	}{
		{"on time", 8, shipmentTick(8), EventContractFulfilled, TrustDeltaContractFulfilled},
		{"early", 8, shipmentTick(7), EventContractFulfilled, TrustDeltaContractFulfilled},
		{"one tick late", 9, shipmentTick(9), EventContractLate, TrustDeltaContractLateOneTick},
		{"two ticks late", 10, shipmentTick(10), EventContractLate, TrustDeltaContractLateTwoTicks},
		{"broken", 11, nil, EventContractBroken, TrustDeltaContractBroken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := relationshipObligation()
			after, err := before.Assess(tt.current, tt.arrival)
			if err != nil {
				t.Fatal(err)
			}
			event, err := ContractOutcome("world", before, after, tt.current)
			if err != nil || event == nil {
				t.Fatalf("event = %+v, %v", event, err)
			}
			if event.Type != tt.wantType || event.TrustDelta != tt.wantDelta {
				t.Fatalf("event type/delta = %s/%d, want %s/%d", event.Type, event.TrustDelta, tt.wantType, tt.wantDelta)
			}
			if event.SourceHouseholdID != "creditor" || event.TargetHouseholdID != "debtor" {
				t.Fatalf("direction = %s -> %s, want creditor -> debtor", event.SourceHouseholdID, event.TargetHouseholdID)
			}
		})
	}

	before := relationshipObligation()
	overdue, err := before.Assess(9, nil)
	if err != nil {
		t.Fatal(err)
	}
	if event, err := ContractOutcome("world", before, overdue, 9); err != nil || event != nil {
		t.Fatalf("overdue event = %+v, %v", event, err)
	}
}

func TestContractOutcomeRejectsInvalidTrustDelta(t *testing.T) {
	before := relationshipObligation()
	after, err := before.Assess(8, shipmentTick(8))
	if err != nil {
		t.Fatal(err)
	}
	event, err := ContractOutcome("world", before, after, 8)
	if err != nil || event == nil {
		t.Fatalf("event = %+v, %v", event, err)
	}
	event.TrustDelta = TrustDeltaContractBroken
	if !errors.Is(event.Validate(), ErrInvalidEvent) {
		t.Fatalf("mismatched trust delta validation = %v", event.Validate())
	}
}

func shipmentTick(value int64) *shipmentdomain.Tick {
	tick := shipmentdomain.Tick(value)
	return &tick
}

func contractTick(value int64) *contractdomain.Tick {
	tick := contractdomain.Tick(value)
	return &tick
}

func TestStandingReflectsAccumulatedContractTrust(t *testing.T) {
	standing, err := StandingForTrust(29 + TrustDeltaContractFulfilled)
	if err != nil || standing != StandingFavorable {
		t.Fatalf("standing after fulfillment = %s, %v; want %s", standing, err, StandingFavorable)
	}
	standing, err = StandingForTrust(-29 + TrustDeltaContractBroken)
	if err != nil || standing != StandingDisapproving {
		t.Fatalf("standing after broken outcome = %s, %v; want %s", standing, err, StandingDisapproving)
	}
}
