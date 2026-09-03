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
	DebtorHouseholdID   string `json:"debtor_household_id"`
	CreditorHouseholdID string `json:"creditor_household_id"`
	ResourceType        string `json:"resource_type"`
	QuantityMilli       int64  `json:"quantity_milli"`
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

type ContractTermProjection struct {
	DebtorHouseholdID   string `json:"debtor_household_id"`
	CreditorHouseholdID string `json:"creditor_household_id"`
	ResourceType        string `json:"resource_type"`
	QuantityMilli       int64  `json:"quantity_milli"`
}

type ContractObligationProjection struct {
	ID                  string  `json:"id"`
	ContractID          string  `json:"contract_id"`
	DebtorHouseholdID   string  `json:"debtor_household_id"`
	CreditorHouseholdID string  `json:"creditor_household_id"`
	ResourceType        string  `json:"resource_type"`
	QuantityMilli       int64   `json:"quantity_milli"`
	DueArrivalTick      int64   `json:"due_arrival_tick"`
	ShipmentID          *string `json:"shipment_id,omitempty"`
	Status              string  `json:"status"`
	FulfilledTick       *int64  `json:"fulfilled_tick,omitempty"`
}

type ContractProjection struct {
	ID                string                         `json:"id"`
	WorldID           string                         `json:"world_id"`
	PartyAHouseholdID string                         `json:"party_a_household_id"`
	PartyBHouseholdID string                         `json:"party_b_household_id"`
	StartsTick        int64                          `json:"starts_tick"`
	EndsTick          int64                          `json:"ends_tick"`
	IntervalTicks     int64                          `json:"interval_ticks"`
	Status            string                         `json:"status"`
	Terms             []ContractTermProjection       `json:"terms"`
	Obligations       []ContractObligationProjection `json:"obligations"`
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

func (s *ContractService) Detail(ctx context.Context, contractID string) (ContractProjection, error) {
	value, err := s.Get(ctx, contractID)
	if err != nil {
		return ContractProjection{}, err
	}
	obligations, err := s.ListObligations(ctx, contractID)
	if err != nil {
		return ContractProjection{}, err
	}
	return projectContract(value, obligations), nil
}

func (s *ContractService) ListDetailsForHousehold(ctx context.Context, householdID string) ([]ContractProjection, error) {
	contracts, err := s.ListForHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	values := make([]ContractProjection, 0, len(contracts))
	for _, value := range contracts {
		obligations, err := s.Store.ListContractObligations(ctx, value.ID)
		if err != nil {
			return nil, err
		}
		values = append(values, projectContract(value, obligations))
	}
	return values, nil
}

func projectContract(value contractdomain.Contract, obligations []contractdomain.Obligation) ContractProjection {
	projection := ContractProjection{
		ID: string(value.ID), WorldID: string(value.WorldID),
		PartyAHouseholdID: string(value.PartyAHouseholdID), PartyBHouseholdID: string(value.PartyBHouseholdID),
		StartsTick: int64(value.StartsTick), EndsTick: int64(value.EndsTick), IntervalTicks: value.IntervalTicks,
		Status: string(value.Status), Terms: make([]ContractTermProjection, 0, len(value.Terms)),
		Obligations: make([]ContractObligationProjection, 0, len(obligations)),
	}
	for _, term := range value.Terms {
		projection.Terms = append(projection.Terms, ContractTermProjection{
			DebtorHouseholdID: string(term.DebtorHouseholdID), CreditorHouseholdID: string(term.CreditorHouseholdID),
			ResourceType: string(term.ResourceType), QuantityMilli: int64(term.QuantityMilli),
		})
	}
	for _, obligation := range obligations {
		item := ContractObligationProjection{
			ID: string(obligation.ID), ContractID: string(obligation.ContractID),
			DebtorHouseholdID: string(obligation.DebtorHouseholdID), CreditorHouseholdID: string(obligation.CreditorHouseholdID),
			ResourceType: string(obligation.ResourceType), QuantityMilli: int64(obligation.QuantityMilli),
			DueArrivalTick: int64(obligation.DueArrivalTick), Status: string(obligation.Status),
		}
		if obligation.ShipmentID != "" {
			shipmentID := string(obligation.ShipmentID)
			item.ShipmentID = &shipmentID
		}
		if obligation.FulfilledTick != nil {
			fulfilled := int64(*obligation.FulfilledTick)
			item.FulfilledTick = &fulfilled
		}
		projection.Obligations = append(projection.Obligations, item)
	}
	return projection
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
