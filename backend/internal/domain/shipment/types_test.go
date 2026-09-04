package shipment

import (
	"errors"
	"testing"
)

func testShipment() Shipment {
	return Shipment{
		ID: "shipment-1", WorldID: "world-1",
		SenderHouseholdID: "sender-1", ReceiverHouseholdID: "receiver-1",
		OriginLocationID: "origin-1", DestinationLocationID: "destination-1",
		ResourceType: "provisions", QuantityMilli: 30_000,
		DepartureTick: 1, ExpectedArrivalTick: 3, TransportCostMilli: 500,
		Status: StatusInTransit,
	}
}

func TestShipmentDoesNotArriveBeforeExpectedTick(t *testing.T) {
	s := testShipment()
	if s.DueAt(2) {
		t.Fatal("shipment reported due before expected arrival")
	}
	if _, err := s.Arrive(2); !errors.Is(err, ErrNotDue) {
		t.Fatalf("Arrive() error = %v, want ErrNotDue", err)
	}
	if s.Status != StatusInTransit || s.ActualArrivalTick != nil {
		t.Fatal("failed arrival mutated shipment")
	}
}

func TestShipmentArrivesExactlyWhenDue(t *testing.T) {
	s := testShipment()
	arrived, err := s.Arrive(3)
	if err != nil {
		t.Fatal(err)
	}
	if arrived.Status != StatusArrived {
		t.Fatalf("status = %q, want %q", arrived.Status, StatusArrived)
	}
	if arrived.ActualArrivalTick == nil || *arrived.ActualArrivalTick != 3 {
		t.Fatalf("actual arrival tick = %v, want 3", arrived.ActualArrivalTick)
	}
	if err := arrived.Validate(); err != nil {
		t.Fatalf("arrived shipment is invalid: %v", err)
	}
}

func TestShipmentArrivalRecordsGameDaySnapshot(t *testing.T) {
	s := testShipment()
	s.DepartureGameDay = 21
	s.ExpectedArrivalGameDay = 35
	arrived, err := s.ArriveAt(3, 36)
	if err != nil {
		t.Fatal(err)
	}
	if arrived.ActualArrivalGameDay == nil || *arrived.ActualArrivalGameDay != 36 {
		t.Fatalf("actual arrival game day = %v, want 36", arrived.ActualArrivalGameDay)
	}
}

func TestCancelledShipmentDoesNotDeliver(t *testing.T) {
	s := testShipment()
	cancelled, err := s.CancelAt(2)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.DueAt(3) {
		t.Fatal("cancelled shipment reported due")
	}
	if _, err := cancelled.Arrive(3); !errors.Is(err, ErrNotDue) {
		t.Fatalf("Arrive() error = %v, want ErrNotDue", err)
	}
}

func TestShipmentCannotBeCancelledOnceDue(t *testing.T) {
	s := testShipment()
	if _, err := s.CancelAt(3); !errors.Is(err, ErrCancellationClosed) {
		t.Fatalf("CancelAt() error = %v, want ErrCancellationClosed", err)
	}
	if _, err := s.Transition(StatusCancelled); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("context-free cancellation error = %v, want ErrInvalidTransition", err)
	}
}

func TestShipmentValidationAndTransitions(t *testing.T) {
	s := testShipment()
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := s
	invalid.QuantityMilli = 0
	if !errors.Is(invalid.Validate(), ErrInvalidShipment) {
		t.Fatal("zero quantity should be invalid")
	}
	invalid = s
	invalid.ExpectedArrivalTick = invalid.DepartureTick
	if !errors.Is(invalid.Validate(), ErrInvalidShipment) {
		t.Fatal("arrival at departure tick should be invalid")
	}
	invalid = s
	invalid.ReceiverHouseholdID = invalid.SenderHouseholdID
	if !errors.Is(invalid.Validate(), ErrInvalidShipment) {
		t.Fatal("same sender and receiver should be invalid")
	}
	if _, err := s.Transition(StatusPrepared); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("transition error = %v, want ErrInvalidTransition", err)
	}
}
