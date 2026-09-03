package application

import (
	"context"
	"errors"
	"testing"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/port"
)

type contractRepositoryStub struct {
	tx         *contractProposalTxStub
	responseTx *contractResponseTxStub
}

func (s contractRepositoryStub) BeginContractProposal(context.Context) (port.ContractProposalTransaction, error) {
	return s.tx, nil
}
func (s contractRepositoryStub) BeginContractResponse(context.Context) (port.ContractResponseTransaction, error) {
	return s.responseTx, nil
}
func (contractRepositoryStub) GetContract(context.Context, contractdomain.ID) (contractdomain.Contract, error) {
	return contractdomain.Contract{}, nil
}
func (contractRepositoryStub) ListContractsForHousehold(context.Context, contractdomain.HouseholdID) ([]contractdomain.Contract, error) {
	return nil, nil
}
func (contractRepositoryStub) ListContractObligations(context.Context, contractdomain.ID) ([]contractdomain.Obligation, error) {
	return nil, nil
}

type contractProposalTxStub struct {
	snapshot  port.ContractPartiesSnapshot
	created   *contractdomain.Contract
	committed bool
}

type contractResponseTxStub struct {
	snapshot     port.ContractResponseSnapshot
	statusWrites int
	obligations  []contractdomain.Obligation
	committed    bool
	createErr    error
}

func (s *contractResponseTxStub) LoadForResponse(context.Context, contractdomain.ID) (port.ContractResponseSnapshot, error) {
	return s.snapshot, nil
}
func (s *contractResponseTxStub) SetStatus(context.Context, contractdomain.ID, contractdomain.Status, contractdomain.Status) error {
	s.statusWrites++
	return nil
}
func (s *contractResponseTxStub) CreateObligations(_ context.Context, values []contractdomain.Obligation) error {
	s.obligations = append(s.obligations, values...)
	return s.createErr
}
func (s *contractResponseTxStub) Commit(context.Context) error   { s.committed = true; return nil }
func (s *contractResponseTxStub) Rollback(context.Context) error { return nil }

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

func proposedContract() contractdomain.Contract {
	return contractdomain.Contract{
		ID: "contract", WorldID: "world", PartyAHouseholdID: "a", PartyBHouseholdID: "b",
		StartsTick: 6, EndsTick: 12, IntervalTicks: 3, Status: contractdomain.StatusProposed,
		Terms: []contractdomain.Term{{DebtorHouseholdID: "a", CreditorHouseholdID: "b", ResourceType: "provisions", QuantityMilli: 10_000}},
	}
}

func TestRespondContractAcceptsAndGeneratesObligationsAtomically(t *testing.T) {
	tx := &contractResponseTxStub{snapshot: port.ContractResponseSnapshot{Contract: proposedContract(), CurrentTick: 5}}
	service := NewContractService(contractRepositoryStub{responseTx: tx})
	updated, err := service.Respond(context.Background(), RespondContractCommand{
		ContractID: "contract", CounterpartyHouseholdID: "b", Accept: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != contractdomain.StatusActive || tx.statusWrites != 1 || len(tx.obligations) != 3 || !tx.committed {
		t.Fatalf("updated=%+v writes=%d obligations=%d committed=%v", updated, tx.statusWrites, len(tx.obligations), tx.committed)
	}
}

func TestRespondContractRejectsUnauthorizedParty(t *testing.T) {
	tx := &contractResponseTxStub{snapshot: port.ContractResponseSnapshot{Contract: proposedContract(), CurrentTick: 5}}
	service := NewContractService(contractRepositoryStub{responseTx: tx})
	_, err := service.Respond(context.Background(), RespondContractCommand{
		ContractID: "contract", CounterpartyHouseholdID: "a", Accept: true,
	})
	if err != ErrContractResponseForbidden || tx.statusWrites != 0 || tx.committed {
		t.Fatalf("error=%v writes=%d committed=%v", err, tx.statusWrites, tx.committed)
	}
}

func TestRespondContractSameDecisionIsIdempotent(t *testing.T) {
	value := proposedContract()
	value.Status = contractdomain.StatusActive
	tx := &contractResponseTxStub{snapshot: port.ContractResponseSnapshot{Contract: value, CurrentTick: 6}}
	service := NewContractService(contractRepositoryStub{responseTx: tx})
	updated, err := service.Respond(context.Background(), RespondContractCommand{
		ContractID: "contract", CounterpartyHouseholdID: "b", Accept: true,
	})
	if err != nil || updated.Status != contractdomain.StatusActive || tx.statusWrites != 0 || len(tx.obligations) != 0 {
		t.Fatalf("updated=%+v error=%v writes=%d obligations=%d", updated, err, tx.statusWrites, len(tx.obligations))
	}
}

func TestRespondContractRejectsWithoutObligations(t *testing.T) {
	tx := &contractResponseTxStub{snapshot: port.ContractResponseSnapshot{Contract: proposedContract(), CurrentTick: 6}}
	service := NewContractService(contractRepositoryStub{responseTx: tx})
	updated, err := service.Respond(context.Background(), RespondContractCommand{
		ContractID: "contract", CounterpartyHouseholdID: "b", Accept: false,
	})
	if err != nil || updated.Status != contractdomain.StatusRejected || tx.statusWrites != 1 || len(tx.obligations) != 0 || !tx.committed {
		t.Fatalf("updated=%+v error=%v writes=%d obligations=%d committed=%v", updated, err, tx.statusWrites, len(tx.obligations), tx.committed)
	}
}

func TestRespondContractRejectsLateAcceptanceBeforeWrite(t *testing.T) {
	tx := &contractResponseTxStub{snapshot: port.ContractResponseSnapshot{Contract: proposedContract(), CurrentTick: 6}}
	service := NewContractService(contractRepositoryStub{responseTx: tx})
	_, err := service.Respond(context.Background(), RespondContractCommand{
		ContractID: "contract", CounterpartyHouseholdID: "b", Accept: true,
	})
	if err != ErrContractStartsInPast || tx.statusWrites != 0 || tx.committed {
		t.Fatalf("error=%v writes=%d committed=%v", err, tx.statusWrites, tx.committed)
	}
}

func TestRespondContractDoesNotCommitPartialActivation(t *testing.T) {
	wantErr := errors.New("obligation write failed")
	tx := &contractResponseTxStub{
		snapshot:  port.ContractResponseSnapshot{Contract: proposedContract(), CurrentTick: 5},
		createErr: wantErr,
	}
	service := NewContractService(contractRepositoryStub{responseTx: tx})
	_, err := service.Respond(context.Background(), RespondContractCommand{
		ContractID: "contract", CounterpartyHouseholdID: "b", Accept: true,
	})
	if !errors.Is(err, wantErr) || tx.statusWrites != 1 || tx.committed {
		t.Fatalf("error=%v writes=%d committed=%v", err, tx.statusWrites, tx.committed)
	}
}
