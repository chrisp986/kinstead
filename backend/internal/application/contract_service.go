package application

import (
	"context"
	"errors"
	"fmt"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/port"
)

var ErrContractStartsInPast = errors.New("contract must start after the current tick")

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
