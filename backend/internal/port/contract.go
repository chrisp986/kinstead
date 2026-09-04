package port

import (
	"context"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/domain/geography"
	shipmentdomain "game/backend/internal/domain/shipment"
)

type ContractPartiesSnapshot struct {
	WorldID        contractdomain.WorldID
	CurrentTick    contractdomain.Tick
	CurrentGameDay contractdomain.GameDay
}

type ContractProposalTransaction interface {
	LoadParties(context.Context, contractdomain.HouseholdID, contractdomain.HouseholdID) (ContractPartiesSnapshot, error)
	Create(context.Context, contractdomain.Contract) (contractdomain.Contract, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type ContractResponseSnapshot struct {
	Contract       contractdomain.Contract
	CurrentTick    contractdomain.Tick
	CurrentGameDay contractdomain.GameDay
}

type ContractResponseTransaction interface {
	LoadForResponse(context.Context, contractdomain.ID) (ContractResponseSnapshot, error)
	SetStatus(context.Context, contractdomain.ID, contractdomain.Status, contractdomain.Status) error
	CreateObligations(context.Context, []contractdomain.Obligation) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type ContractDispatchSnapshot struct {
	Obligation            contractdomain.Obligation
	WorldID               contractdomain.WorldID
	ContractStatus        contractdomain.Status
	OriginLocationID      shipmentdomain.LocationID
	DestinationLocationID shipmentdomain.LocationID
	Route                 geography.Route
	CurrentTick           contractdomain.Tick
	CurrentGameDay        contractdomain.GameDay
	GameDaysPerTickNum    int64
	GameDaysPerTickDen    int64
	GameDaySchedule       bool
	ProposedShipmentID    shipmentdomain.ID
	ExistingShipment      *shipmentdomain.Shipment
}

type ContractDispatchTransaction interface {
	LoadForDispatch(context.Context, contractdomain.ObligationID, contractdomain.HouseholdID) (ContractDispatchSnapshot, error)
	PersistDispatch(context.Context, contractdomain.Obligation, contractdomain.Obligation, shipmentdomain.Shipment) (shipmentdomain.Shipment, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type ContractRepository interface {
	BeginContractProposal(context.Context) (ContractProposalTransaction, error)
	BeginContractResponse(context.Context) (ContractResponseTransaction, error)
	BeginContractDispatch(context.Context) (ContractDispatchTransaction, error)
	GetContract(context.Context, contractdomain.ID) (contractdomain.Contract, error)
	ListContractsForHousehold(context.Context, contractdomain.HouseholdID) ([]contractdomain.Contract, error)
	ListContractObligations(context.Context, contractdomain.ID) ([]contractdomain.Obligation, error)
}
