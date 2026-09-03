package contract

import (
	"errors"
	"testing"

	shipmentdomain "game/backend/internal/domain/shipment"
)

func activeContract() Contract {
	return Contract{
		ID: "contract", WorldID: "world", PartyAHouseholdID: "farm-a", PartyBHouseholdID: "farm-b",
		StartsTick: 4, EndsTick: 10, IntervalTicks: 3, Status: StatusActive,
		Terms: []Term{{DebtorHouseholdID: "farm-a", CreditorHouseholdID: "farm-b", ResourceType: "provisions", QuantityMilli: 10_000}},
	}
}

func pendingObligation() Obligation {
	return Obligation{ID: "obligation", ContractID: "contract", DebtorHouseholdID: "farm-a", CreditorHouseholdID: "farm-b", ResourceType: "provisions", QuantityMilli: 10_000, DueArrivalTick: 8, Status: ObligationPending}
}

func matchingShipment() shipmentdomain.Shipment {
	return shipmentdomain.Shipment{ID: "shipment", WorldID: "world", SenderHouseholdID: "farm-a", ReceiverHouseholdID: "farm-b", OriginLocationID: "a", DestinationLocationID: "b", ResourceType: "provisions", QuantityMilli: 10_000, DepartureTick: 5, ExpectedArrivalTick: 8, Status: shipmentdomain.StatusInTransit}
}

func TestGenerateRecurringObligationsAtInclusiveIntervals(t *testing.T) {
	got, err := GenerateObligations(activeContract())
	if err != nil {
		t.Fatal(err)
	}
	want := []Tick{4, 7, 10}
	if len(got) != len(want) {
		t.Fatalf("obligations = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].DueArrivalTick != want[i] || got[i].Status != ObligationPending {
			t.Fatalf("obligation %d = %+v", i, got[i])
		}
	}
}

func TestContractTransitions(t *testing.T) {
	proposed := activeContract()
	proposed.Status = StatusProposed
	active, err := proposed.Transition(StatusActive)
	if err != nil || active.Status != StatusActive {
		t.Fatalf("activate = %+v, %v", active, err)
	}
	if _, err := proposed.Transition(StatusCompleted); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("invalid transition error = %v", err)
	}
}

func TestContractRejectsInvalidOrDuplicateTerms(t *testing.T) {
	value := activeContract()
	value.Terms = append(value.Terms, value.Terms[0])
	if !errors.Is(value.Validate(), ErrInvalidContract) {
		t.Fatal("duplicate terms should be invalid")
	}
	value = activeContract()
	value.Terms[0].CreditorHouseholdID = "outsider"
	if !errors.Is(value.Validate(), ErrInvalidContract) {
		t.Fatal("non-party term should be invalid")
	}
}

func TestContractRollUpRequiresCompleteSettledSchedule(t *testing.T) {
	value := activeContract()
	obligations, err := GenerateObligations(value)
	if err != nil {
		t.Fatal(err)
	}
	for i := range obligations {
		obligations[i].ID = ObligationID(string(rune('a' + i)))
		obligations[i].ShipmentID = shipmentdomain.ID(string(rune('A' + i)))
		fulfilled := obligations[i].DueArrivalTick
		obligations[i].FulfilledTick = &fulfilled
		obligations[i].Status = ObligationFulfilled
	}
	completed, err := value.RollUp(obligations)
	if err != nil || completed.Status != StatusCompleted {
		t.Fatalf("completed rollup = %+v, %v", completed, err)
	}
	obligations[1].Status = ObligationBroken
	lateArrival := obligations[1].DueArrivalTick + 3
	obligations[1].FulfilledTick = &lateArrival
	broken, err := value.RollUp(obligations)
	if err != nil || broken.Status != StatusBroken {
		t.Fatalf("broken rollup = %+v, %v", broken, err)
	}
	if _, err := value.RollUp(obligations[:2]); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("incomplete schedule error = %v", err)
	}
}

func TestDispatchRequiresMatchingPhysicalShipment(t *testing.T) {
	dispatched, err := pendingObligation().Dispatch(matchingShipment())
	if err != nil || dispatched.Status != ObligationDispatched || dispatched.ShipmentID != "shipment" {
		t.Fatalf("dispatch = %+v, %v", dispatched, err)
	}
	mismatch := matchingShipment()
	mismatch.QuantityMilli--
	if _, err := pendingObligation().Dispatch(mismatch); !errors.Is(err, ErrShipmentMismatch) {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestLateObligationCanStillDispatchBeforeBroken(t *testing.T) {
	late, err := pendingObligation().Assess(9, nil)
	if err != nil {
		t.Fatal(err)
	}
	shipment := matchingShipment()
	shipment.DepartureTick = 9
	shipment.ExpectedArrivalTick = 10
	dispatched, err := late.Dispatch(shipment)
	if err != nil || dispatched.Status != ObligationLate || dispatched.ShipmentID == "" {
		t.Fatalf("late dispatch = %+v, %v", dispatched, err)
	}
}

func TestObligationAssessmentUsesActualArrival(t *testing.T) {
	dispatched, err := pendingObligation().Dispatch(matchingShipment())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		current Tick
		arrival *shipmentdomain.Tick
		want    ObligationStatus
	}{
		{"not due", 7, nil, ObligationDispatched},
		{"on time", 8, shipmentTick(8), ObligationFulfilled},
		{"one late", 9, shipmentTick(9), ObligationLate},
		{"two late", 10, shipmentTick(10), ObligationLate},
		{"three late", 11, shipmentTick(11), ObligationBroken},
		{"waiting late", 10, nil, ObligationLate},
		{"waiting broken", 11, nil, ObligationBroken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dispatched.Assess(tt.current, tt.arrival)
			if err != nil || got.Status != tt.want {
				t.Fatalf("assessment = %+v, %v; want %s", got, err, tt.want)
			}
		})
	}
}

func TestBrokenObligationRecordsEventualArrival(t *testing.T) {
	dispatched, err := pendingObligation().Dispatch(matchingShipment())
	if err != nil {
		t.Fatal(err)
	}
	broken, err := dispatched.Assess(11, nil)
	if err != nil || broken.Status != ObligationBroken || broken.FulfilledTick != nil {
		t.Fatalf("broken assessment = %+v, %v", broken, err)
	}
	arrived, err := broken.Assess(12, shipmentTick(12))
	if err != nil || arrived.Status != ObligationBroken || arrived.FulfilledTick == nil || *arrived.FulfilledTick != 12 {
		t.Fatalf("eventual arrival = %+v, %v", arrived, err)
	}
}

func shipmentTick(value shipmentdomain.Tick) *shipmentdomain.Tick { return &value }
