package contract

import (
	"errors"
	"math"

	"game/backend/internal/calendar"
	shipmentdomain "game/backend/internal/domain/shipment"
)

type ID string
type ObligationID string
type WorldID string
type HouseholdID string
type ResourceType string
type QuantityMilli int64
type Tick int64
type GameDay = calendar.GameDay
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
	StartGameDay      GameDay
	EndGameDay        GameDay
	IntervalDays      int64
	GameDaySchedule   bool
	Status            Status
	Terms             []Term
}

func (c Contract) Validate() error {
	if c.WorldID == "" || c.PartyAHouseholdID == "" || c.PartyBHouseholdID == "" ||
		c.PartyAHouseholdID == c.PartyBHouseholdID || len(c.Terms) == 0 {
		return ErrInvalidContract
	}
	if c.IntervalDays > 0 {
		if c.StartGameDay < 0 || c.EndGameDay < c.StartGameDay || c.IntervalDays > math.MaxInt32 {
			return ErrInvalidContract
		}
	} else if c.StartsTick < 0 || c.EndsTick < c.StartsTick || c.IntervalTicks <= 0 || c.IntervalTicks > math.MaxInt32 {
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

// RollUp derives an active contract's terminal lifecycle from its complete,
// persistence-backed obligation schedule. A single broken obligation breaks
// the contract; otherwise every obligation must have actually settled before
// the contract completes.
func (c Contract) RollUp(obligations []Obligation) (Contract, error) {
	if err := c.Validate(); err != nil || c.ID == "" || c.Status != StatusActive {
		return Contract{}, ErrInvalidContract
	}
	expected, err := GenerateObligations(c)
	if err != nil || len(obligations) != len(expected) {
		return Contract{}, ErrInvalidContract
	}
	type obligationKey struct {
		Debtor   HouseholdID
		Creditor HouseholdID
		Resource ResourceType
		Quantity QuantityMilli
		Due      int64
	}
	expectedKeys := make(map[obligationKey]struct{}, len(expected))
	for _, obligation := range expected {
		expectedKeys[obligationKey{
			Debtor: obligation.DebtorHouseholdID, Creditor: obligation.CreditorHouseholdID,
			Resource: obligation.ResourceType, Quantity: obligation.QuantityMilli, Due: obligationDue(obligation),
		}] = struct{}{}
	}
	allSettled := true
	for _, obligation := range obligations {
		if err := obligation.Validate(); err != nil || obligation.ID == "" || obligation.ContractID != c.ID {
			return Contract{}, ErrInvalidContract
		}
		key := obligationKey{
			Debtor: obligation.DebtorHouseholdID, Creditor: obligation.CreditorHouseholdID,
			Resource: obligation.ResourceType, Quantity: obligation.QuantityMilli, Due: obligationDue(obligation),
		}
		if _, ok := expectedKeys[key]; !ok {
			return Contract{}, ErrInvalidContract
		}
		delete(expectedKeys, key)
		if obligation.Status == ObligationBroken {
			return c.Transition(StatusBroken)
		}
		settled := obligation.Status == ObligationFulfilled ||
			(obligation.Status == ObligationLate && ((c.IntervalDays > 0 && obligation.FulfilledGameDay != nil) || (c.IntervalDays <= 0 && obligation.FulfilledTick != nil)))
		allSettled = allSettled && settled
	}
	if len(expectedKeys) != 0 {
		return Contract{}, ErrInvalidContract
	}
	if allSettled {
		return c.Transition(StatusCompleted)
	}
	return c, nil
}

type Obligation struct {
	ID                     ObligationID
	ContractID             ID
	DebtorHouseholdID      HouseholdID
	CreditorHouseholdID    HouseholdID
	ResourceType           ResourceType
	QuantityMilli          QuantityMilli
	DueArrivalTick         Tick
	DueGameDay             GameDay
	ShipmentID             shipmentdomain.ID
	ExpectedArrivalGameDay *GameDay
	LatestDispatchGameDay  *GameDay
	Status                 ObligationStatus
	FulfilledTick          *Tick
	FulfilledGameDay       *GameDay
}

func obligationDue(o Obligation) int64 {
	if o.DueGameDay != 0 || o.DueArrivalTick == 0 {
		return int64(o.DueGameDay)
	}
	return int64(o.DueArrivalTick)
}

// GenerateObligations expands an active contract using its authoritative
// game-day schedule. The zero-origin clock is retained for compatibility with
// pure domain callers; persistence-backed application code should call
// GenerateObligationsAt with the world's current clock state.
func GenerateObligations(c Contract) ([]Obligation, error) {
	if !c.GameDaySchedule || c.IntervalDays <= 0 {
		if err := c.Validate(); err != nil || c.ID == "" || c.Status != StatusActive {
			return nil, ErrInvalidContract
		}
		obligations := make([]Obligation, 0)
		for due := c.StartsTick; due <= c.EndsTick; {
			for _, term := range c.Terms {
				obligations = append(obligations, Obligation{
					ContractID: c.ID, DebtorHouseholdID: term.DebtorHouseholdID, CreditorHouseholdID: term.CreditorHouseholdID,
					ResourceType: term.ResourceType, QuantityMilli: term.QuantityMilli,
					DueArrivalTick: due, Status: ObligationPending,
				})
			}
			if due > c.EndsTick-Tick(c.IntervalTicks) {
				break
			}
			due += Tick(c.IntervalTicks)
		}
		return obligations, nil
	}
	return GenerateObligationsAt(c, 0, 0, 0, calendar.DaysPerSeason, 12)
}

// GenerateObligationsAt expands a contract and derives execution ticks from
// the supplied world clock. DueGameDay is the business deadline; DueArrivalTick
// is only an execution/index projection for the tick worker.
func GenerateObligationsAt(c Contract, currentTick int64, currentDay GameDay, remainder, numerator, denominator int64) ([]Obligation, error) {
	if err := c.Validate(); err != nil || c.ID == "" || c.Status != StatusActive {
		return nil, ErrInvalidContract
	}
	if c.IntervalDays <= 0 || currentTick < 0 || currentDay < 0 {
		return nil, ErrInvalidContract
	}
	obligations := make([]Obligation, 0)
	start, end, interval := int64(c.StartGameDay), int64(c.EndGameDay), c.IntervalDays
	for due := start; due <= end; {
		dueTickOffset, err := calendar.TicksUntilGameDay(currentDay, remainder, numerator, denominator, GameDay(due))
		if err != nil || dueTickOffset > math.MaxInt64-currentTick {
			return nil, ErrInvalidContract
		}
		for _, term := range c.Terms {
			obligation := Obligation{
				ContractID: c.ID, DebtorHouseholdID: term.DebtorHouseholdID, CreditorHouseholdID: term.CreditorHouseholdID,
				ResourceType: term.ResourceType, QuantityMilli: term.QuantityMilli,
				DueArrivalTick: Tick(currentTick + dueTickOffset), DueGameDay: GameDay(due), Status: ObligationPending,
			}
			obligations = append(obligations, obligation)
		}
		if due > math.MaxInt64-interval || due+interval > end {
			break
		}
		due += interval
	}
	return obligations, nil
}

func (o Obligation) Validate() error {
	if o.ContractID == "" || o.DebtorHouseholdID == "" || o.CreditorHouseholdID == "" ||
		o.DebtorHouseholdID == o.CreditorHouseholdID || o.ResourceType == "" || o.QuantityMilli <= 0 || (o.DueArrivalTick < 0 && o.DueGameDay < 0) {
		return ErrInvalidObligation
	}
	if o.DueArrivalTick > Tick(math.MaxInt64-3) {
		return ErrInvalidObligation
	}
	if o.DueGameDay > GameDay(math.MaxInt64-3) {
		return ErrInvalidObligation
	}
	dayBased := o.DueGameDay != 0 || o.FulfilledGameDay != nil
	switch o.Status {
	case ObligationPending:
		if o.ShipmentID != "" || o.FulfilledTick != nil || o.FulfilledGameDay != nil {
			return ErrInvalidObligation
		}
	case ObligationDispatched:
		if o.ShipmentID == "" || o.FulfilledTick != nil || o.FulfilledGameDay != nil {
			return ErrInvalidObligation
		}
	case ObligationFulfilled:
		if o.ShipmentID == "" {
			return ErrInvalidObligation
		}
		if dayBased {
			if o.FulfilledGameDay == nil || *o.FulfilledGameDay > o.DueGameDay {
				return ErrInvalidObligation
			}
		} else if o.FulfilledTick == nil || *o.FulfilledTick > o.DueArrivalTick {
			return ErrInvalidObligation
		}
	case ObligationLate:
		if dayBased {
			if o.FulfilledGameDay != nil && (o.ShipmentID == "" || *o.FulfilledGameDay <= o.DueGameDay || *o.FulfilledGameDay > o.DueGameDay+14) {
				return ErrInvalidObligation
			}
		} else if o.FulfilledTick != nil && (o.ShipmentID == "" || *o.FulfilledTick <= o.DueArrivalTick || *o.FulfilledTick > o.DueArrivalTick+2) {
			return ErrInvalidObligation
		}
	case ObligationBroken:
		if dayBased {
			if o.FulfilledGameDay != nil && (o.ShipmentID == "" || *o.FulfilledGameDay < o.DueGameDay+15) {
				return ErrInvalidObligation
			}
		} else if o.FulfilledTick != nil && (o.ShipmentID == "" || *o.FulfilledTick < o.DueArrivalTick+3) {
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
	if o.DueGameDay != 0 && shipment.DepartureGameDay > shipmentdomain.GameDay(o.DueGameDay) {
		o.Status = ObligationLate
	} else if o.DueGameDay == 0 && Tick(shipment.DepartureTick) > o.DueArrivalTick {
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
	if o.Status == ObligationFulfilled || (o.Status == ObligationLate && o.FulfilledTick != nil) ||
		(o.Status == ObligationBroken && (o.FulfilledTick != nil || actualArrival == nil)) {
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

// AssessGameDay applies the contract's day-based delivery buckets: on time is
// day 0, one through seven days late is late, eight through fourteen days late
// is late2, and fifteen or more days late is broken. Existing callers using
// tick-backed obligations continue to use Assess above until their storage
// projection is upgraded.
func (o Obligation) AssessGameDay(current GameDay, actual *GameDay) (Obligation, error) {
	if o.DueGameDay < 0 || current < 0 || o.ContractID == "" || o.ShipmentID == "" && actual != nil {
		return Obligation{}, ErrInvalidObligation
	}
	if o.Status == ObligationFulfilled || (o.Status == ObligationLate && o.FulfilledGameDay != nil) ||
		(o.Status == ObligationBroken && (o.FulfilledGameDay != nil || actual == nil)) {
		return o, nil
	}
	if actual != nil {
		if *actual < 0 {
			return Obligation{}, ErrInvalidObligation
		}
		o.FulfilledGameDay = actual
		delay := *actual - o.DueGameDay
		switch {
		case delay <= 0:
			o.Status = ObligationFulfilled
		case delay <= 14:
			o.Status = ObligationLate
		default:
			o.Status = ObligationBroken
		}
		return o, nil
	}
	if current <= o.DueGameDay {
		return o, nil
	}
	if current-o.DueGameDay <= 14 {
		o.Status = ObligationLate
	} else {
		o.Status = ObligationBroken
	}
	return o, nil
}
