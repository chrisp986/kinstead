package contract

import (
	"errors"
	"math"

	shipmentdomain "game/backend/internal/domain/shipment"
)

type ID string
type ObligationID string
type WorldID string
type HouseholdID string
type ResourceType string
type QuantityMilli int64
type Tick int64
type Status string
type ObligationStatus string

const (
	StatusProposed  Status = "proposed"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusRejected  Status = "rejected"
	StatusCancelled Status = "cancelled"
	StatusBroken    Status = "broken"

	ObligationPending    ObligationStatus = "pending"
	ObligationDispatched ObligationStatus = "dispatched"
	ObligationFulfilled  ObligationStatus = "fulfilled"
	ObligationLate       ObligationStatus = "late"
	ObligationBroken     ObligationStatus = "broken"
)

var (
	ErrInvalidContract   = errors.New("invalid contract")
	ErrInvalidObligation = errors.New("invalid contract obligation")
	ErrInvalidTransition = errors.New("invalid contract status transition")
	ErrShipmentMismatch  = errors.New("shipment does not match contract obligation")
)

type Term struct {
	DebtorHouseholdID   HouseholdID
	CreditorHouseholdID HouseholdID
	ResourceType        ResourceType
	QuantityMilli       QuantityMilli
}

type Contract struct {
	ID                ID
	WorldID           WorldID
	PartyAHouseholdID HouseholdID
	PartyBHouseholdID HouseholdID
	StartsTick        Tick
	EndsTick          Tick
	IntervalTicks     int64
	Status            Status
	Terms             []Term
}

func (c Contract) Validate() error {
	if c.WorldID == "" || c.PartyAHouseholdID == "" || c.PartyBHouseholdID == "" ||
		c.PartyAHouseholdID == c.PartyBHouseholdID || c.StartsTick < 0 || c.EndsTick < c.StartsTick ||
		c.IntervalTicks <= 0 || c.IntervalTicks > math.MaxInt32 || len(c.Terms) == 0 {
		return ErrInvalidContract
	}
	switch c.Status {
	case StatusProposed, StatusActive, StatusCompleted, StatusRejected, StatusCancelled, StatusBroken:
	default:
		return ErrInvalidContract
	}
	seenTerms := make(map[Term]struct{}, len(c.Terms))
	for _, term := range c.Terms {
		if term.QuantityMilli <= 0 || term.ResourceType == "" ||
			!c.hasParty(term.DebtorHouseholdID) || !c.hasParty(term.CreditorHouseholdID) ||
			term.DebtorHouseholdID == term.CreditorHouseholdID {
			return ErrInvalidContract
		}
		if _, duplicate := seenTerms[term]; duplicate {
			return ErrInvalidContract
		}
		seenTerms[term] = struct{}{}
	}
	return nil
}

func (c Contract) hasParty(id HouseholdID) bool {
	return id == c.PartyAHouseholdID || id == c.PartyBHouseholdID
}

func (c Contract) Transition(to Status) (Contract, error) {
	valid := false
	switch c.Status {
	case StatusProposed:
		valid = to == StatusActive || to == StatusRejected || to == StatusCancelled
	case StatusActive:
		valid = to == StatusCompleted || to == StatusCancelled || to == StatusBroken
	}
	if !valid {
		return Contract{}, ErrInvalidTransition
	}
	c.Status = to
	return c, nil
}

type Obligation struct {
	ID                  ObligationID
	ContractID          ID
	DebtorHouseholdID   HouseholdID
	CreditorHouseholdID HouseholdID
	ResourceType        ResourceType
	QuantityMilli       QuantityMilli
	DueArrivalTick      Tick
	ShipmentID          shipmentdomain.ID
	Status              ObligationStatus
	FulfilledTick       *Tick
}

// GenerateObligations expands every active contract term at starts_tick and
// each inclusive interval through ends_tick. IDs are assigned by persistence.
func GenerateObligations(c Contract) ([]Obligation, error) {
	if err := c.Validate(); err != nil || c.ID == "" || c.Status != StatusActive {
		return nil, ErrInvalidContract
	}
	obligations := make([]Obligation, 0)
	for due := int64(c.StartsTick); due <= int64(c.EndsTick); {
		for _, term := range c.Terms {
			obligations = append(obligations, Obligation{
				ContractID: c.ID, DebtorHouseholdID: term.DebtorHouseholdID, CreditorHouseholdID: term.CreditorHouseholdID,
				ResourceType: term.ResourceType, QuantityMilli: term.QuantityMilli,
				DueArrivalTick: Tick(due), Status: ObligationPending,
			})
		}
		if due > math.MaxInt64-c.IntervalTicks || due+c.IntervalTicks > int64(c.EndsTick) {
			break
		}
		due += c.IntervalTicks
	}
	return obligations, nil
}

func (o Obligation) Validate() error {
	if o.ContractID == "" || o.DebtorHouseholdID == "" || o.CreditorHouseholdID == "" ||
		o.DebtorHouseholdID == o.CreditorHouseholdID || o.ResourceType == "" || o.QuantityMilli <= 0 || o.DueArrivalTick < 0 {
		return ErrInvalidObligation
	}
	if o.DueArrivalTick > Tick(math.MaxInt64-3) {
		return ErrInvalidObligation
	}
	switch o.Status {
	case ObligationPending:
		if o.ShipmentID != "" || o.FulfilledTick != nil {
			return ErrInvalidObligation
		}
	case ObligationDispatched:
		if o.ShipmentID == "" || o.FulfilledTick != nil {
			return ErrInvalidObligation
		}
	case ObligationFulfilled:
		if o.ShipmentID == "" || o.FulfilledTick == nil || *o.FulfilledTick > o.DueArrivalTick {
			return ErrInvalidObligation
		}
	case ObligationLate:
		if o.FulfilledTick != nil && (o.ShipmentID == "" || *o.FulfilledTick <= o.DueArrivalTick || *o.FulfilledTick > o.DueArrivalTick+2) {
			return ErrInvalidObligation
		}
	case ObligationBroken:
		if o.FulfilledTick != nil && (o.ShipmentID == "" || *o.FulfilledTick < o.DueArrivalTick+3) {
			return ErrInvalidObligation
		}
	default:
		return ErrInvalidObligation
	}
	return nil
}

func (o Obligation) Dispatch(shipment shipmentdomain.Shipment) (Obligation, error) {
	if (o.Status != ObligationPending && o.Status != ObligationLate) || o.ShipmentID != "" || shipment.ID == "" ||
		shipment.SenderHouseholdID != shipmentdomain.HouseholdID(o.DebtorHouseholdID) ||
		shipment.ReceiverHouseholdID != shipmentdomain.HouseholdID(o.CreditorHouseholdID) ||
		shipment.ResourceType != shipmentdomain.ResourceType(o.ResourceType) ||
		shipment.QuantityMilli != shipmentdomain.QuantityMilli(o.QuantityMilli) ||
		(shipment.Status != shipmentdomain.StatusInTransit && shipment.Status != shipmentdomain.StatusArrived) {
		return Obligation{}, ErrShipmentMismatch
	}
	o.ShipmentID = shipment.ID
	if Tick(shipment.DepartureTick) > o.DueArrivalTick {
		o.Status = ObligationLate
	} else {
		o.Status = ObligationDispatched
	}
	return o, nil
}

// Assess determines fulfillment strictly from actual arrival. One or two
// ticks after the due tick is late; three or more is broken.
func (o Obligation) Assess(currentTick Tick, actualArrival *shipmentdomain.Tick) (Obligation, error) {
	if err := o.Validate(); err != nil || currentTick < 0 {
		return Obligation{}, ErrInvalidObligation
	}
	if o.Status == ObligationFulfilled || o.Status == ObligationBroken || (o.Status == ObligationLate && o.FulfilledTick != nil) {
		return o, nil
	}
	if actualArrival != nil {
		arrival := Tick(*actualArrival)
		if o.ShipmentID == "" || arrival < 0 {
			return Obligation{}, ErrInvalidObligation
		}
		o.FulfilledTick = &arrival
		delay := arrival - o.DueArrivalTick
		switch {
		case delay <= 0:
			o.Status = ObligationFulfilled
		case delay <= 2:
			o.Status = ObligationLate
		default:
			o.Status = ObligationBroken
		}
		return o, nil
	}
	if currentTick <= o.DueArrivalTick {
		return o, nil
	}
	if currentTick-o.DueArrivalTick <= 2 {
		o.Status = ObligationLate
	} else {
		o.Status = ObligationBroken
	}
	return o, nil
}
