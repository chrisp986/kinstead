//go:build postgres

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	contractdomain "game/backend/internal/domain/contract"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
	sqlcdb "game/backend/internal/postgres/db"
)

var ErrInvalidContractParticipants = errors.New("contract participants must be distinct households in one world")

type contractProposalTx struct {
	store    *Store
	tx       pgx.Tx
	partyA   contractdomain.HouseholdID
	partyB   contractdomain.HouseholdID
	snapshot *port.ContractPartiesSnapshot
}

type contractResponseTx struct {
	tx pgx.Tx
}

func (s *Store) BeginContractProposal(ctx context.Context) (port.ContractProposalTransaction, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &contractProposalTx{store: s, tx: tx}, nil
}

func (s *Store) BeginContractResponse(ctx context.Context) (port.ContractResponseTransaction, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &contractResponseTx{tx: tx}, nil
}

func (t *contractProposalTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *contractProposalTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
func (t *contractResponseTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *contractResponseTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

func (t *contractProposalTx) LoadParties(ctx context.Context, partyA, partyB contractdomain.HouseholdID) (port.ContractPartiesSnapshot, error) {
	if partyA == "" || partyB == "" || partyA == partyB {
		return port.ContractPartiesSnapshot{}, ErrInvalidContractParticipants
	}
	var snapshot port.ContractPartiesSnapshot
	err := t.tx.QueryRow(ctx, `
		SELECT a.world_id::text, w.current_tick
		FROM households a
		JOIN households b ON b.id = $2::uuid AND b.world_id = a.world_id
		JOIN worlds w ON w.id = a.world_id
		WHERE a.id = $1::uuid
		FOR UPDATE OF w
	`, partyA, partyB).Scan(&snapshot.WorldID, &snapshot.CurrentTick)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.ContractPartiesSnapshot{}, ErrInvalidContractParticipants
	}
	if err == nil {
		t.partyA, t.partyB = partyA, partyB
		t.snapshot = &snapshot
	}
	return snapshot, err
}

func (t *contractProposalTx) Create(ctx context.Context, value contractdomain.Contract) (contractdomain.Contract, error) {
	if err := value.Validate(); err != nil || value.Status != contractdomain.StatusProposed || value.ID != "" ||
		t.snapshot == nil || value.WorldID != t.snapshot.WorldID ||
		value.PartyAHouseholdID != t.partyA || value.PartyBHouseholdID != t.partyB {
		return contractdomain.Contract{}, contractdomain.ErrInvalidContract
	}
	worldID, err := uuidParam(string(value.WorldID))
	if err != nil {
		return contractdomain.Contract{}, err
	}
	partyA, err := uuidParam(string(value.PartyAHouseholdID))
	if err != nil {
		return contractdomain.Contract{}, err
	}
	partyB, err := uuidParam(string(value.PartyBHouseholdID))
	if err != nil {
		return contractdomain.Contract{}, err
	}
	queries := sqlcdb.New(t.tx)
	row, err := queries.CreateContract(ctx, sqlcdb.CreateContractParams{
		Column1: worldID, Column2: partyA, Column3: partyB,
		StartsTick: int64(value.StartsTick), EndsTick: int64(value.EndsTick),
		IntervalTicks: int32(value.IntervalTicks), Status: string(value.Status),
	})
	if err != nil {
		return contractdomain.Contract{}, err
	}
	contractID, err := uuidParam(row.ID)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	for _, term := range value.Terms {
		debtorID, err := uuidParam(string(term.DebtorHouseholdID))
		if err != nil {
			return contractdomain.Contract{}, err
		}
		creditorID, err := uuidParam(string(term.CreditorHouseholdID))
		if err != nil {
			return contractdomain.Contract{}, err
		}
		if err := queries.CreateContractTerm(ctx, sqlcdb.CreateContractTermParams{
			Column1: contractID, Column2: debtorID, Column3: creditorID,
			ResourceCode: string(term.ResourceType), QuantityMilli: int64(term.QuantityMilli),
		}); err != nil {
			return contractdomain.Contract{}, err
		}
	}
	value.ID = contractdomain.ID(row.ID)
	return value, nil
}

func (s *Store) GetContract(ctx context.Context, id contractdomain.ID) (contractdomain.Contract, error) {
	contractID, err := uuidParam(string(id))
	if err != nil {
		return contractdomain.Contract{}, err
	}
	queries := sqlcdb.New(s.Pool)
	row, err := queries.GetContract(ctx, contractID)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	return loadContractTerms(ctx, queries, contractFromRow(row.ID, row.WorldID, row.PartyAHouseholdID,
		row.PartyBHouseholdID, row.StartsTick, row.EndsTick, row.IntervalTicks, row.Status))
}

func (s *Store) ListContractsForHousehold(ctx context.Context, householdID contractdomain.HouseholdID) ([]contractdomain.Contract, error) {
	id, err := uuidParam(string(householdID))
	if err != nil {
		return nil, err
	}
	queries := sqlcdb.New(s.Pool)
	rows, err := queries.ListContractsForHousehold(ctx, id)
	if err != nil {
		return nil, err
	}
	values := make([]contractdomain.Contract, 0, len(rows))
	for _, row := range rows {
		value, err := loadContractTerms(ctx, queries, contractFromRow(row.ID, row.WorldID, row.PartyAHouseholdID,
			row.PartyBHouseholdID, row.StartsTick, row.EndsTick, row.IntervalTicks, row.Status))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (t *contractResponseTx) LoadForResponse(ctx context.Context, id contractdomain.ID) (port.ContractResponseSnapshot, error) {
	contractID, err := uuidParam(string(id))
	if err != nil {
		return port.ContractResponseSnapshot{}, err
	}
	queries := sqlcdb.New(t.tx)
	row, err := queries.LockContractForResponse(ctx, contractID)
	if err != nil {
		return port.ContractResponseSnapshot{}, err
	}
	value := contractFromRow(row.ID, row.WorldID, row.PartyAHouseholdID, row.PartyBHouseholdID,
		row.StartsTick, row.EndsTick, row.IntervalTicks, row.Status)
	value, err = loadContractTerms(ctx, queries, value)
	if err != nil {
		return port.ContractResponseSnapshot{}, err
	}
	return port.ContractResponseSnapshot{Contract: value, CurrentTick: contractdomain.Tick(row.CurrentTick)}, nil
}

func (t *contractResponseTx) SetStatus(ctx context.Context, id contractdomain.ID, from, to contractdomain.Status) error {
	contractID, err := uuidParam(string(id))
	if err != nil {
		return err
	}
	rows, err := sqlcdb.New(t.tx).UpdateContractStatus(ctx, sqlcdb.UpdateContractStatusParams{
		Column1: contractID, Status: string(to), Status_2: string(from),
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return contractdomain.ErrInvalidTransition
	}
	return nil
}

func (t *contractResponseTx) CreateObligations(ctx context.Context, obligations []contractdomain.Obligation) error {
	queries := sqlcdb.New(t.tx)
	for _, obligation := range obligations {
		if err := obligation.Validate(); err != nil || obligation.ID != "" || obligation.Status != contractdomain.ObligationPending {
			return contractdomain.ErrInvalidObligation
		}
		contractID, err := uuidParam(string(obligation.ContractID))
		if err != nil {
			return err
		}
		debtorID, err := uuidParam(string(obligation.DebtorHouseholdID))
		if err != nil {
			return err
		}
		creditorID, err := uuidParam(string(obligation.CreditorHouseholdID))
		if err != nil {
			return err
		}
		if err := queries.CreateContractObligation(ctx, sqlcdb.CreateContractObligationParams{
			Column1: contractID, Column2: debtorID, Column3: creditorID,
			ResourceCode: string(obligation.ResourceType), QuantityMilli: int64(obligation.QuantityMilli),
			DueArrivalTick: int64(obligation.DueArrivalTick), Status: string(obligation.Status),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListContractObligations(ctx context.Context, contractID contractdomain.ID) ([]contractdomain.Obligation, error) {
	id, err := uuidParam(string(contractID))
	if err != nil {
		return nil, err
	}
	rows, err := sqlcdb.New(s.Pool).ListContractObligations(ctx, id)
	if err != nil {
		return nil, err
	}
	values := make([]contractdomain.Obligation, 0, len(rows))
	for _, row := range rows {
		value := contractdomain.Obligation{
			ID: contractdomain.ObligationID(row.ID), ContractID: contractdomain.ID(row.ContractID),
			DebtorHouseholdID:   contractdomain.HouseholdID(row.DebtorHouseholdID),
			CreditorHouseholdID: contractdomain.HouseholdID(row.CreditorHouseholdID),
			ResourceType:        contractdomain.ResourceType(row.ResourceCode), QuantityMilli: contractdomain.QuantityMilli(row.QuantityMilli),
			DueArrivalTick: contractdomain.Tick(row.DueArrivalTick), Status: contractdomain.ObligationStatus(row.Status),
		}
		if row.ShipmentID != "" {
			value.ShipmentID = shipmentdomain.ID(row.ShipmentID)
		}
		if row.FulfilledTick.Valid {
			fulfilled := contractdomain.Tick(row.FulfilledTick.Int64)
			value.FulfilledTick = &fulfilled
		}
		if err := value.Validate(); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func loadContractTerms(ctx context.Context, queries *sqlcdb.Queries, value contractdomain.Contract) (contractdomain.Contract, error) {
	id, err := uuidParam(string(value.ID))
	if err != nil {
		return contractdomain.Contract{}, err
	}
	rows, err := queries.ListContractTerms(ctx, id)
	if err != nil {
		return contractdomain.Contract{}, err
	}
	value.Terms = make([]contractdomain.Term, 0, len(rows))
	for _, row := range rows {
		value.Terms = append(value.Terms, contractdomain.Term{
			DebtorHouseholdID:   contractdomain.HouseholdID(row.DebtorHouseholdID),
			CreditorHouseholdID: contractdomain.HouseholdID(row.CreditorHouseholdID),
			ResourceType:        contractdomain.ResourceType(row.ResourceCode), QuantityMilli: contractdomain.QuantityMilli(row.QuantityMilli),
		})
	}
	if err := value.Validate(); err != nil {
		return contractdomain.Contract{}, err
	}
	return value, nil
}

func contractFromRow(id, worldID, partyA, partyB string, starts, ends int64, interval int32, status string) contractdomain.Contract {
	return contractdomain.Contract{
		ID: contractdomain.ID(id), WorldID: contractdomain.WorldID(worldID),
		PartyAHouseholdID: contractdomain.HouseholdID(partyA), PartyBHouseholdID: contractdomain.HouseholdID(partyB),
		StartsTick: contractdomain.Tick(starts), EndsTick: contractdomain.Tick(ends), IntervalTicks: int64(interval),
		Status: contractdomain.Status(status),
	}
}
