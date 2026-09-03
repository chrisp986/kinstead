package port

import (
	"context"

	contractdomain "game/backend/internal/domain/contract"
)

type ContractPartiesSnapshot struct {
	WorldID     contractdomain.WorldID
	CurrentTick contractdomain.Tick
}

type ContractProposalTransaction interface {
	LoadParties(context.Context, contractdomain.HouseholdID, contractdomain.HouseholdID) (ContractPartiesSnapshot, error)
	Create(context.Context, contractdomain.Contract) (contractdomain.Contract, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type ContractResponseSnapshot struct {
	Contract    contractdomain.Contract
	CurrentTick contractdomain.Tick
}

type ContractResponseTransaction interface {
	LoadForResponse(context.Context, contractdomain.ID) (ContractResponseSnapshot, error)
	SetStatus(context.Context, contractdomain.ID, contractdomain.Status, contractdomain.Status) error
	CreateObligations(context.Context, []contractdomain.Obligation) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type ContractRepository interface {
	BeginContractProposal(context.Context) (ContractProposalTransaction, error)
	BeginContractResponse(context.Context) (ContractResponseTransaction, error)
	GetContract(context.Context, contractdomain.ID) (contractdomain.Contract, error)
	ListContractsForHousehold(context.Context, contractdomain.HouseholdID) ([]contractdomain.Contract, error)
	ListContractObligations(context.Context, contractdomain.ID) ([]contractdomain.Obligation, error)
}
