package relationship

import (
	"errors"
	"math"

	contractdomain "game/backend/internal/domain/contract"
	shipmentdomain "game/backend/internal/domain/shipment"
)

type EventType string
type Standing string

const (
	EventContractFulfilled EventType = "contract_obligation_fulfilled"
	EventContractLate      EventType = "contract_obligation_late"
	EventContractBroken    EventType = "contract_obligation_broken"
)

const (
	TrustDeltaContractFulfilled    = 2
	TrustDeltaContractLateOneTick  = -1
	TrustDeltaContractLateTwoTicks = -2
	TrustDeltaContractBroken       = -8
)

const (
	StandingDisapproving Standing = "disapproving"
	StandingNeutral      Standing = "neutral"
	StandingFavorable    Standing = "favorable"
	StandingConnected    Standing = "connected"
)

var ErrInvalidEvent = errors.New("invalid relationship event")
var ErrInvalidTrust = errors.New("relationship trust must be between -100 and 100")

// TrustDeltaForContractOutcome maps one final contract-obligation outcome to
// the directional trust consequence for the creditor's view of the debtor.
// A late outcome must include its actual arrival so the one- and two-tick
// penalties remain deterministic. A broken outcome may record an eventual
// arrival at least three ticks late, or no arrival at all.
func TrustDeltaForContractOutcome(eventType EventType, dueTick contractdomain.Tick, fulfilledTick *contractdomain.Tick) (int, error) {
	if dueTick < 0 || dueTick > contractdomain.Tick(math.MaxInt64-3) {
		return 0, ErrInvalidEvent
	}
	switch eventType {
	case EventContractFulfilled:
		if fulfilledTick == nil || *fulfilledTick < 0 || *fulfilledTick > dueTick {
			return 0, ErrInvalidEvent
		}
		return TrustDeltaContractFulfilled, nil
	case EventContractLate:
		if fulfilledTick == nil || *fulfilledTick <= dueTick {
			return 0, ErrInvalidEvent
		}
		switch *fulfilledTick - dueTick {
		case 1:
			return TrustDeltaContractLateOneTick, nil
		case 2:
			return TrustDeltaContractLateTwoTicks, nil
		default:
			return 0, ErrInvalidEvent
		}
	case EventContractBroken:
		if fulfilledTick != nil && (*fulfilledTick < 0 || *fulfilledTick < dueTick+3) {
			return 0, ErrInvalidEvent
		}
		return TrustDeltaContractBroken, nil
	default:
		return 0, ErrInvalidEvent
	}
}

func StandingForTrust(trust int) (Standing, error) {
	switch {
	case trust < -100 || trust > 100:
		return "", ErrInvalidTrust
	case trust <= -31:
		return StandingDisapproving, nil
	case trust <= 29:
		return StandingNeutral, nil
	case trust <= 69:
		return StandingFavorable, nil
	default:
		return StandingConnected, nil
	}
}

type Event struct {
	WorldID               contractdomain.WorldID
	SourceHouseholdID     contractdomain.HouseholdID
	TargetHouseholdID     contractdomain.HouseholdID
	Type                  EventType
	TrustDelta            int
	OccurredTick          contractdomain.Tick
	ContractID            contractdomain.ID
	ObligationID          contractdomain.ObligationID
	ShipmentID            shipmentdomain.ID
	ResourceType          contractdomain.ResourceType
	QuantityMilli         contractdomain.QuantityMilli
	DueArrivalTick        contractdomain.Tick
	ActualFulfillmentTick *contractdomain.Tick
}

func (e Event) Validate() error {
	if e.WorldID == "" || e.SourceHouseholdID == "" || e.TargetHouseholdID == "" ||
		e.SourceHouseholdID == e.TargetHouseholdID || e.ContractID == "" || e.ObligationID == "" ||
		e.ResourceType == "" || e.QuantityMilli <= 0 || e.DueArrivalTick < 0 || e.OccurredTick < 0 ||
		e.TrustDelta < -100 || e.TrustDelta > 100 {
		return ErrInvalidEvent
	}
	switch e.Type {
	case EventContractFulfilled:
		if e.ShipmentID == "" || e.ActualFulfillmentTick == nil || *e.ActualFulfillmentTick > e.DueArrivalTick {
			return ErrInvalidEvent
		}
	case EventContractLate:
		if e.ShipmentID == "" || e.ActualFulfillmentTick == nil || *e.ActualFulfillmentTick <= e.DueArrivalTick || *e.ActualFulfillmentTick > e.DueArrivalTick+2 {
			return ErrInvalidEvent
		}
	case EventContractBroken:
		if e.OccurredTick < e.DueArrivalTick+3 || (e.ActualFulfillmentTick != nil && (e.ShipmentID == "" || *e.ActualFulfillmentTick < e.DueArrivalTick+3)) {
			return ErrInvalidEvent
		}
	default:
		return ErrInvalidEvent
	}
	if e.ActualFulfillmentTick != nil && e.OccurredTick != *e.ActualFulfillmentTick {
		return ErrInvalidEvent
	}
	delta, err := TrustDeltaForContractOutcome(e.Type, e.DueArrivalTick, e.ActualFulfillmentTick)
	if err != nil || e.TrustDelta != delta {
		return ErrInvalidEvent
	}
	return nil
}

// ContractOutcome emits only final relationship-relevant outcomes. Merely
// becoming overdue is not an outcome: it becomes late when goods actually
// arrive within two ticks, or broken once the third tick passes.
func ContractOutcome(worldID contractdomain.WorldID, before, after contractdomain.Obligation, currentTick contractdomain.Tick) (*Event, error) {
	if err := before.Validate(); err != nil {
		return nil, err
	}
	if err := after.Validate(); err != nil {
		return nil, err
	}
	if before.ID == "" || before.ID != after.ID || before.ContractID != after.ContractID ||
		before.DebtorHouseholdID != after.DebtorHouseholdID || before.CreditorHouseholdID != after.CreditorHouseholdID ||
		before.ResourceType != after.ResourceType || before.QuantityMilli != after.QuantityMilli ||
		before.DueArrivalTick != after.DueArrivalTick || currentTick < 0 {
		return nil, ErrInvalidEvent
	}
	var eventType EventType
	var occurred contractdomain.Tick
	switch {
	case after.Status == contractdomain.ObligationFulfilled && before.Status != contractdomain.ObligationFulfilled:
		eventType, occurred = EventContractFulfilled, *after.FulfilledTick
	case after.Status == contractdomain.ObligationLate && after.FulfilledTick != nil && before.FulfilledTick == nil:
		eventType, occurred = EventContractLate, *after.FulfilledTick
	case after.Status == contractdomain.ObligationBroken && before.Status != contractdomain.ObligationBroken:
		eventType, occurred = EventContractBroken, currentTick
	default:
		return nil, nil
	}
	event := &Event{
		WorldID: worldID, SourceHouseholdID: after.CreditorHouseholdID, TargetHouseholdID: after.DebtorHouseholdID,
		Type: eventType, OccurredTick: occurred, ContractID: after.ContractID, ObligationID: after.ID,
		ShipmentID: after.ShipmentID, ResourceType: after.ResourceType, QuantityMilli: after.QuantityMilli,
		DueArrivalTick: after.DueArrivalTick, ActualFulfillmentTick: after.FulfilledTick,
	}
	delta, err := TrustDeltaForContractOutcome(eventType, after.DueArrivalTick, after.FulfilledTick)
	if err != nil {
		return nil, err
	}
	event.TrustDelta = delta
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return event, nil
}
