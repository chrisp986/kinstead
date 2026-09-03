package relationship

import (
	"errors"

	contractdomain "game/backend/internal/domain/contract"
	shipmentdomain "game/backend/internal/domain/shipment"
)

type EventType string

const (
	EventContractFulfilled EventType = "contract_obligation_fulfilled"
	EventContractLate      EventType = "contract_obligation_late"
	EventContractBroken    EventType = "contract_obligation_broken"
)

var ErrInvalidEvent = errors.New("invalid relationship event")

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
		if e.ActualFulfillmentTick != nil && (e.ShipmentID == "" || *e.ActualFulfillmentTick < e.DueArrivalTick+3) {
			return ErrInvalidEvent
		}
	default:
		return ErrInvalidEvent
	}
	if e.ActualFulfillmentTick != nil && e.OccurredTick != *e.ActualFulfillmentTick {
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
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return event, nil
}
