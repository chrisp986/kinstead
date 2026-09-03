package application

import (
	"context"
	"testing"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/port"
)

type contractRepositoryStub struct{ tx *contractProposalTxStub }

func (s contractRepositoryStub) BeginContractProposal(context.Context) (port.ContractProposalTransaction, error) {
	return s.tx, nil
}
func (contractRepositoryStub) GetContract(context.Context, contractdomain.ID) (contractdomain.Contract, error) {
	return contractdomain.Contract{}, nil
}
func (contractRepositoryStub) ListContractsForHousehold(context.Context, contractdomain.HouseholdID) ([]contractdomain.Contract, error) {
	return nil, nil
}

type contractProposalTxStub struct {
	snapshot  port.ContractPartiesSnapshot
	created   *contractdomain.Contract
	committed bool
}

func (s *contractProposalTxStub) LoadParties(context.Context, contractdomain.HouseholdID, contractdomain.HouseholdID) (port.ContractPartiesSnapshot, error) {
	return s.snapshot, nil
}
func (s *contractProposalTxStub) Create(_ context.Context, value contractdomain.Contract) (contractdomain.Contract, error) {
	value.ID = "created"
	s.created = &value
	return value, nil
}
func (s *contractProposalTxStub) Commit(context.Context) error   { s.committed = true; return nil }
func (s *contractProposalTxStub) Rollback(context.Context) error { return nil }

func validProposalCommand() ProposeContractCommand {
	return ProposeContractCommand{
		ProposerHouseholdID: "a", CounterpartyHouseholdID: "b",
		StartsTick: 6, EndsTick: 12, IntervalTicks: 3,
		Terms: []ContractTermIntent{{DebtorHouseholdID: "a", CreditorHouseholdID: "b", ResourceType: "provisions", QuantityMilli: 10_000}},
	}
}

func TestProposeContractUsesAuthoritativeWorldAndTick(t *testing.T) {
	tx := &contractProposalTxStub{snapshot: port.ContractPartiesSnapshot{WorldID: "world", CurrentTick: 5}}
	service := NewContractService(contractRepositoryStub{tx: tx})
	created, err := service.Propose(context.Background(), validProposalCommand())
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "created" || created.WorldID != "world" || created.Status != contractdomain.StatusProposed || !tx.committed {
		t.Fatalf("proposal = %+v, committed=%v", created, tx.committed)
	}
}

func TestProposeContractRejectsProcessedStartTickBeforeWrite(t *testing.T) {
	tx := &contractProposalTxStub{snapshot: port.ContractPartiesSnapshot{WorldID: "world", CurrentTick: 6}}
	service := NewContractService(contractRepositoryStub{tx: tx})
	if _, err := service.Propose(context.Background(), validProposalCommand()); err != ErrContractStartsInPast {
		t.Fatalf("error = %v, want ErrContractStartsInPast", err)
	}
	if tx.created != nil || tx.committed {
		t.Fatal("invalid proposal reached persistence")
	}
}
