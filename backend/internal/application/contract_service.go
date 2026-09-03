package application

import (
	"context"
	"errors"
	"fmt"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/domain/geography"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
)

var ErrContractStartsInPast = errors.New("contract must start after the current tick")
var ErrContractResponseForbidden = errors.New("only the proposed counterparty may respond to a contract")
var ErrContractDispatchForbidden = errors.New("only the obligation debtor may dispatch its shipment")

type ContractTermIntent struct {
	DebtorHouseholdID   string
	CreditorHouseholdID string
	ResourceType        string
	QuantityMilli       int64
}

type ProposeContractCommand struct {
	ProposerHouseholdID     string
	CounterpartyHouseholdID string
	StartsTick              int64
	EndsTick                int64
	IntervalTicks           int64
	Terms                   []ContractTermIntent
}

type RespondContractCommand struct {
	ContractID              string
	CounterpartyHouseholdID string
	Accept                  bool
}

type DispatchContractObligationCommand struct {
	ObligationID      string
	DebtorHouseholdID string
}

type DispatchContractObligationResult struct {
	Obligation contractdomain.Obligation
	Shipment   shipmentdomain.Shipment
}

type ContractService struct {
	Store port.ContractRepository
}

func NewContractService(store port.ContractRepository) *ContractService {
	return &ContractService{Store: store}
}

func (s *ContractService) Propose(ctx context.Context, cmd ProposeContractCommand) (contractdomain.Contract, error) {
	tx, err := s.Store.BeginContractProposal(ctx)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	defer tx.Rollback(ctx)

	partyA := contractdomain.HouseholdID(cmd.ProposerHouseholdID)
	partyB := contractdomain.HouseholdID(cmd.CounterpartyHouseholdID)
	snapshot, err := tx.LoadParties(ctx, partyA, partyB)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	if contractdomain.Tick(cmd.StartsTick) <= snapshot.CurrentTick {
		return contractdomain.Contract{}, ErrContractStartsInPast
	}
	terms := make([]contractdomain.Term, 0, len(cmd.Terms))
	for _, term := range cmd.Terms {
		terms = append(terms, contractdomain.Term{
			DebtorHouseholdID:   contractdomain.HouseholdID(term.DebtorHouseholdID),
			CreditorHouseholdID: contractdomain.HouseholdID(term.CreditorHouseholdID),
			ResourceType:        contractdomain.ResourceType(term.ResourceType), QuantityMilli: contractdomain.QuantityMilli(term.QuantityMilli),
		})
	}
	proposal := contractdomain.Contract{
		WorldID: snapshot.WorldID, PartyAHouseholdID: partyA, PartyBHouseholdID: partyB,
		StartsTick: contractdomain.Tick(cmd.StartsTick), EndsTick: contractdomain.Tick(cmd.EndsTick),
		IntervalTicks: cmd.IntervalTicks, Status: contractdomain.StatusProposed, Terms: terms,
	}
	if err := proposal.Validate(); err != nil {
		return contractdomain.Contract{}, err
	}
	created, err := tx.Create(ctx, proposal)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contractdomain.Contract{}, fmt.Errorf("commit contract proposal: %w", err)
	}
	return created, nil
}

func (s *ContractService) Get(ctx context.Context, id string) (contractdomain.Contract, error) {
	return s.Store.GetContract(ctx, contractdomain.ID(id))
}

func (s *ContractService) ListForHousehold(ctx context.Context, householdID string) ([]contractdomain.Contract, error) {
	return s.Store.ListContractsForHousehold(ctx, contractdomain.HouseholdID(householdID))
}

func (s *ContractService) ListObligations(ctx context.Context, contractID string) ([]contractdomain.Obligation, error) {
	return s.Store.ListContractObligations(ctx, contractdomain.ID(contractID))
}

// Respond accepts or rejects a proposal as its counterparty. Repeating the
// same decision is idempotent; an accepted contract and all of its recurring
// obligations are committed atomically.
func (s *ContractService) Respond(ctx context.Context, cmd RespondContractCommand) (contractdomain.Contract, error) {
	tx, err := s.Store.BeginContractResponse(ctx)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	defer tx.Rollback(ctx)

	snapshot, err := tx.LoadForResponse(ctx, contractdomain.ID(cmd.ContractID))
	if err != nil {
		return contractdomain.Contract{}, err
	}
	value := snapshot.Contract
	if contractdomain.HouseholdID(cmd.CounterpartyHouseholdID) != value.PartyBHouseholdID {
		return contractdomain.Contract{}, ErrContractResponseForbidden
	}
	target := contractdomain.StatusRejected
	if cmd.Accept {
		target = contractdomain.StatusActive
	}
	if value.Status == target {
		return value, nil
	}
	if cmd.Accept && value.StartsTick <= snapshot.CurrentTick {
		return contractdomain.Contract{}, ErrContractStartsInPast
	}
	updated, err := value.Transition(target)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	if err := tx.SetStatus(ctx, value.ID, value.Status, target); err != nil {
		return contractdomain.Contract{}, err
	}
	if cmd.Accept {
		obligations, err := contractdomain.GenerateObligations(updated)
		if err != nil {
			return contractdomain.Contract{}, err
		}
		if err := tx.CreateObligations(ctx, obligations); err != nil {
			return contractdomain.Contract{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return contractdomain.Contract{}, fmt.Errorf("commit contract response: %w", err)
	}
	return updated, nil
}

// DispatchObligation reserves the promised goods, creates their physical
// shipment using authoritative geography, and links it to the obligation in a
// single transaction. The obligation itself is the idempotency key.
func (s *ContractService) DispatchObligation(ctx context.Context, cmd DispatchContractObligationCommand) (DispatchContractObligationResult, error) {
	tx, err := s.Store.BeginContractDispatch(ctx)
	if err != nil {
		return DispatchContractObligationResult{}, err
	}
	defer tx.Rollback(ctx)

	snapshot, err := tx.LoadForDispatch(ctx, contractdomain.ObligationID(cmd.ObligationID), contractdomain.HouseholdID(cmd.DebtorHouseholdID))
	if err != nil {
		return DispatchContractObligationResult{}, err
	}
	if snapshot.Obligation.DebtorHouseholdID != contractdomain.HouseholdID(cmd.DebtorHouseholdID) {
		return DispatchContractObligationResult{}, ErrContractDispatchForbidden
	}
	if snapshot.ExistingShipment != nil {
		return DispatchContractObligationResult{Obligation: snapshot.Obligation, Shipment: *snapshot.ExistingShipment}, nil
	}
	if snapshot.ContractStatus != contractdomain.StatusActive {
		return DispatchContractObligationResult{}, contractdomain.ErrInvalidTransition
	}
	arrival, err := geography.ArrivalTick(geography.Tick(snapshot.CurrentTick), snapshot.Route.TravelTicks)
	if err != nil {
		return DispatchContractObligationResult{}, err
	}
	prepared := shipmentdomain.Shipment{
		ID:                    snapshot.ProposedShipmentID,
		WorldID:               shipmentdomain.WorldID(snapshot.WorldID),
		SenderHouseholdID:     shipmentdomain.HouseholdID(snapshot.Obligation.DebtorHouseholdID),
		ReceiverHouseholdID:   shipmentdomain.HouseholdID(snapshot.Obligation.CreditorHouseholdID),
		OriginLocationID:      snapshot.OriginLocationID,
		DestinationLocationID: snapshot.DestinationLocationID,
		ResourceType:          shipmentdomain.ResourceType(snapshot.Obligation.ResourceType),
		QuantityMilli:         shipmentdomain.QuantityMilli(snapshot.Obligation.QuantityMilli),
		DepartureTick:         shipmentdomain.Tick(snapshot.CurrentTick),
		ExpectedArrivalTick:   shipmentdomain.Tick(arrival),
		TransportCostMilli:    shipmentdomain.MoneyMilli(snapshot.Route.TransportCostMilli),
		Status:                shipmentdomain.StatusPrepared,
	}
	shipment, err := prepared.Transition(shipmentdomain.StatusInTransit)
	if err != nil {
		return DispatchContractObligationResult{}, err
	}
	updated, err := snapshot.Obligation.Dispatch(shipment)
	if err != nil {
		return DispatchContractObligationResult{}, err
	}
	created, err := tx.PersistDispatch(ctx, snapshot.Obligation, updated, shipment)
	if err != nil {
		return DispatchContractObligationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DispatchContractObligationResult{}, fmt.Errorf("commit contract dispatch: %w", err)
	}
	updated.ShipmentID = created.ID
	return DispatchContractObligationResult{Obligation: updated, Shipment: created}, nil
}
