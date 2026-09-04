//go:build postgres

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/domain/geography"
	relationshipdomain "game/backend/internal/domain/relationship"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
	sqlcdb "game/backend/internal/postgres/db"
)

var ErrInvalidContractParticipants = errors.New("contract participants must be distinct households in one world")
var ErrContractDispatchStateChanged = errors.New("contract obligation changed during dispatch")

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

type contractDispatchTx struct {
	store *Store
	tx    pgx.Tx
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

func (s *Store) BeginContractDispatch(ctx context.Context) (port.ContractDispatchTransaction, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &contractDispatchTx{store: s, tx: tx}, nil
}

func (t *contractProposalTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *contractProposalTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
func (t *contractResponseTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *contractResponseTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }
func (t *contractDispatchTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *contractDispatchTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

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
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM households WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, pgx.ErrNoRows
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
	return contractObligationsFromRows(rows)
}

func contractObligationsFromRows(rows []sqlcdb.ListContractObligationsRow) ([]contractdomain.Obligation, error) {
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

func (s *Store) LoadContractObligationsForTick(ctx context.Context, tx pgx.Tx, worldID string, tick int64) ([]port.ContractObligationAssessment, error) {
	id, err := uuidParam(worldID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcdb.New(tx).LoadContractObligationsForTick(ctx, sqlcdb.LoadContractObligationsForTickParams{
		Column1: id, DueArrivalTick: tick,
	})
	if err != nil {
		return nil, err
	}
	values := make([]port.ContractObligationAssessment, 0, len(rows))
	for _, row := range rows {
		value := contractdomain.Obligation{
			ID: contractdomain.ObligationID(row.ID), ContractID: contractdomain.ID(row.ContractID),
			DebtorHouseholdID:   contractdomain.HouseholdID(row.DebtorHouseholdID),
			CreditorHouseholdID: contractdomain.HouseholdID(row.CreditorHouseholdID),
			ResourceType:        contractdomain.ResourceType(row.ResourceCode), QuantityMilli: contractdomain.QuantityMilli(row.QuantityMilli),
			DueArrivalTick: contractdomain.Tick(row.DueArrivalTick), ShipmentID: shipmentdomain.ID(row.ShipmentID),
			Status: contractdomain.ObligationStatus(row.Status),
		}
		if row.FulfilledTick.Valid {
			fulfilled := contractdomain.Tick(row.FulfilledTick.Int64)
			value.FulfilledTick = &fulfilled
		}
		if err := value.Validate(); err != nil {
			return nil, fmt.Errorf("contract obligation %s: %w", value.ID, err)
		}
		assessment := port.ContractObligationAssessment{WorldID: contractdomain.WorldID(row.WorldID), Obligation: value}
		if row.ActualArrivalTick.Valid {
			arrival := shipmentdomain.Tick(row.ActualArrivalTick.Int64)
			assessment.ActualArrivalTick = &arrival
		}
		values = append(values, assessment)
	}
	return values, nil
}

func (s *Store) PersistContractObligationAssessment(ctx context.Context, tx pgx.Tx, before, after contractdomain.Obligation, event *relationshipdomain.Event) (bool, error) {
	if err := before.Validate(); err != nil {
		return false, err
	}
	if err := after.Validate(); err != nil {
		return false, err
	}
	if before.ID == "" || before.ID != after.ID || before.ContractID != after.ContractID ||
		before.DebtorHouseholdID != after.DebtorHouseholdID || before.CreditorHouseholdID != after.CreditorHouseholdID ||
		before.ResourceType != after.ResourceType || before.QuantityMilli != after.QuantityMilli ||
		before.DueArrivalTick != after.DueArrivalTick || before.ShipmentID != after.ShipmentID {
		return false, contractdomain.ErrInvalidObligation
	}
	id, err := uuidParam(string(after.ID))
	if err != nil {
		return false, err
	}
	rows, err := sqlcdb.New(tx).UpdateContractObligationAssessment(ctx, sqlcdb.UpdateContractObligationAssessmentParams{
		NewStatus:        string(after.Status),
		FulfilledTick:    nullableContractTick(after.FulfilledTick),
		ID:               id,
		OldStatus:        string(before.Status),
		OldFulfilledTick: nullableContractTick(before.FulfilledTick),
	})
	if err != nil || rows != 1 {
		return rows == 1, err
	}
	if event != nil {
		if err := persistRelationshipEvent(ctx, tx, *event); err != nil {
			return false, err
		}
		if err := persistContractOutcomeChronicle(ctx, tx, *event); err != nil {
			return false, err
		}
	}
	return true, nil
}

// persistContractOutcomeChronicle records the same final outcome observed by
// the relationship projection for both affected households. The obligation
// reference and unique index make retries idempotent.
func persistContractOutcomeChronicle(ctx context.Context, tx pgx.Tx, event relationshipdomain.Event) error {
	entryType := string(event.Type)
	data := map[string]any{
		"resource_type":    string(event.ResourceType),
		"quantity_milli":   int64(event.QuantityMilli),
		"due_arrival_tick": int64(event.DueArrivalTick),
		"trust_delta":      event.TrustDelta,
		"contract_id":      string(event.ContractID),
		"obligation_id":    string(event.ObligationID),
	}
	if event.ActualFulfillmentTick != nil {
		data["actual_arrival_tick"] = int64(*event.ActualFulfillmentTick)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	for _, household := range []struct {
		id      string
		related string
	}{
		{string(event.SourceHouseholdID), string(event.TargetHouseholdID)},
		{string(event.TargetHouseholdID), string(event.SourceHouseholdID)},
	} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO chronicle_entries(
				household_id, occurred_tick, entry_type, related_household_id,
				related_contract_id, related_obligation_id, data
			) VALUES ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6::uuid, $7::jsonb)
			ON CONFLICT (household_id, related_obligation_id, entry_type)
			WHERE related_obligation_id IS NOT NULL DO NOTHING
		`, household.id, event.OccurredTick, entryType, household.related,
			string(event.ContractID), string(event.ObligationID), payload); err != nil {
			return err
		}
	}
	return nil
}

func persistRelationshipEvent(ctx context.Context, tx pgx.Tx, event relationshipdomain.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	worldID, err := uuidParam(string(event.WorldID))
	if err != nil {
		return err
	}
	sourceID, err := uuidParam(string(event.SourceHouseholdID))
	if err != nil {
		return err
	}
	targetID, err := uuidParam(string(event.TargetHouseholdID))
	if err != nil {
		return err
	}
	contractID, err := uuidParam(string(event.ContractID))
	if err != nil {
		return err
	}
	obligationID, err := uuidParam(string(event.ObligationID))
	if err != nil {
		return err
	}
	queries := sqlcdb.New(tx)
	rows, err := queries.InsertRelationshipEvent(ctx, sqlcdb.InsertRelationshipEventParams{
		WorldID: worldID, SourceHouseholdID: sourceID, TargetHouseholdID: targetID,
		EventType: string(event.Type), TrustDelta: int32(event.TrustDelta), OccurredTick: int64(event.OccurredTick),
		ContractID: contractID, ShipmentID: string(event.ShipmentID), ObligationID: obligationID,
		ResourceType: string(event.ResourceType), QuantityMilli: int64(event.QuantityMilli), DueArrivalTick: int64(event.DueArrivalTick),
		ActualFulfillmentTick: nullableContractTick(event.ActualFulfillmentTick),
	})
	if err != nil || rows == 0 {
		return err
	}
	return queries.ApplyRelationshipDelta(ctx, sqlcdb.ApplyRelationshipDeltaParams{
		WorldID: worldID, SourceHouseholdID: sourceID, TargetHouseholdID: targetID,
		TrustDelta: int32(event.TrustDelta), OccurredTick: pgtype.Int8{Int64: int64(event.OccurredTick), Valid: true},
	})
}

func (s *Store) LoadActiveContractsForRollup(ctx context.Context, tx pgx.Tx, worldID string) ([]port.ContractRollupSnapshot, error) {
	id, err := uuidParam(worldID)
	if err != nil {
		return nil, err
	}
	queries := sqlcdb.New(tx)
	rows, err := queries.ListActiveContractsForRollup(ctx, id)
	if err != nil {
		return nil, err
	}
	values := make([]port.ContractRollupSnapshot, 0, len(rows))
	for _, row := range rows {
		value := contractFromRow(row.ID, row.WorldID, row.PartyAHouseholdID, row.PartyBHouseholdID,
			row.StartsTick, row.EndsTick, row.IntervalTicks, row.Status)
		value, err = loadContractTerms(ctx, queries, value)
		if err != nil {
			return nil, err
		}
		contractID, err := uuidParam(row.ID)
		if err != nil {
			return nil, err
		}
		obligationRows, err := queries.ListContractObligations(ctx, contractID)
		if err != nil {
			return nil, err
		}
		obligations, err := contractObligationsFromRows(obligationRows)
		if err != nil {
			return nil, err
		}
		values = append(values, port.ContractRollupSnapshot{Contract: value, Obligations: obligations})
	}
	return values, nil
}

func (s *Store) PersistContractRollup(ctx context.Context, tx pgx.Tx, before, after contractdomain.Contract) (bool, error) {
	if err := before.Validate(); err != nil {
		return false, err
	}
	if err := after.Validate(); err != nil {
		return false, err
	}
	if !sameContractExceptStatus(before, after) {
		return false, contractdomain.ErrInvalidContract
	}
	expected, err := before.Transition(after.Status)
	if err != nil || expected.Status != after.Status {
		return false, contractdomain.ErrInvalidTransition
	}
	id, err := uuidParam(string(before.ID))
	if err != nil {
		return false, err
	}
	rows, err := sqlcdb.New(tx).UpdateContractStatus(ctx, sqlcdb.UpdateContractStatusParams{
		Column1: id, Status: string(after.Status), Status_2: string(before.Status),
	})
	return rows == 1, err
}

func sameContractExceptStatus(a, b contractdomain.Contract) bool {
	if a.ID != b.ID || a.ID == "" || a.WorldID != b.WorldID || a.PartyAHouseholdID != b.PartyAHouseholdID ||
		a.PartyBHouseholdID != b.PartyBHouseholdID || a.StartsTick != b.StartsTick || a.EndsTick != b.EndsTick ||
		a.IntervalTicks != b.IntervalTicks || len(a.Terms) != len(b.Terms) {
		return false
	}
	for i := range a.Terms {
		if a.Terms[i] != b.Terms[i] {
			return false
		}
	}
	return true
}

func (t *contractDispatchTx) LoadForDispatch(ctx context.Context, obligationID contractdomain.ObligationID, _ contractdomain.HouseholdID) (port.ContractDispatchSnapshot, error) {
	id, err := uuidParam(string(obligationID))
	if err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	queries := sqlcdb.New(t.tx)
	row, err := queries.LockContractObligationForDispatch(ctx, id)
	if err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	obligation := contractdomain.Obligation{
		ID: contractdomain.ObligationID(row.ID), ContractID: contractdomain.ID(row.ContractID),
		DebtorHouseholdID: contractdomain.HouseholdID(row.DebtorHouseholdID), CreditorHouseholdID: contractdomain.HouseholdID(row.CreditorHouseholdID),
		ResourceType: contractdomain.ResourceType(row.ResourceCode), QuantityMilli: contractdomain.QuantityMilli(row.QuantityMilli),
		DueArrivalTick: contractdomain.Tick(row.DueArrivalTick), ShipmentID: shipmentdomain.ID(row.ShipmentID),
		Status: contractdomain.ObligationStatus(row.Status),
	}
	if row.FulfilledTick.Valid {
		fulfilled := contractdomain.Tick(row.FulfilledTick.Int64)
		obligation.FulfilledTick = &fulfilled
	}
	if err := obligation.Validate(); err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	snapshot := port.ContractDispatchSnapshot{
		Obligation: obligation, WorldID: contractdomain.WorldID(row.WorldID), ContractStatus: contractdomain.Status(row.ContractStatus),
		OriginLocationID: shipmentdomain.LocationID(row.OriginLocationID), DestinationLocationID: shipmentdomain.LocationID(row.DestinationLocationID),
		CurrentTick: contractdomain.Tick(row.CurrentTick), ProposedShipmentID: shipmentdomain.ID(row.ProposedShipmentID),
	}
	if obligation.ShipmentID != "" {
		shipment, err := scanShipment(t.tx.QueryRow(ctx, `
			SELECT id::text, world_id::text, sender_household_id::text, receiver_household_id::text,
			       origin_location_id::text, destination_location_id::text, resource_code,
			       quantity_milli, departure_tick, expected_arrival_tick,
			       actual_arrival_tick, transport_cost_milli, status
			FROM shipments WHERE id = $1::uuid
		`, obligation.ShipmentID))
		if err != nil {
			return port.ContractDispatchSnapshot{}, err
		}
		snapshot.ExistingShipment = &shipment
		return snapshot, nil
	}
	worldID, err := uuidParam(row.WorldID)
	if err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	originID, err := uuidParam(row.OriginLocationID)
	if err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	destinationID, err := uuidParam(row.DestinationLocationID)
	if err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	distance, err := queries.GetRouteDistance(ctx, sqlcdb.GetRouteDistanceParams{
		Column1: worldID, Column2: originID, Column3: destinationID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return port.ContractDispatchSnapshot{}, geography.ErrRouteUnavailable
	}
	if err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	route, err := geography.RouteForDistance(
		geography.WorldID(row.WorldID), geography.LocationID(row.OriginLocationID),
		geography.LocationID(row.DestinationLocationID), geography.DistanceClass(distance),
	)
	if err != nil {
		return port.ContractDispatchSnapshot{}, err
	}
	snapshot.Route = route
	return snapshot, nil
}

func (t *contractDispatchTx) PersistDispatch(ctx context.Context, before, after contractdomain.Obligation, shipment shipmentdomain.Shipment) (shipmentdomain.Shipment, error) {
	if err := before.Validate(); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if err := after.Validate(); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if err := shipment.Validate(); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if before.ID == "" || before.ID != after.ID || before.ContractID != after.ContractID ||
		before.DebtorHouseholdID != after.DebtorHouseholdID || before.CreditorHouseholdID != after.CreditorHouseholdID ||
		before.ResourceType != after.ResourceType || before.QuantityMilli != after.QuantityMilli ||
		before.DueArrivalTick != after.DueArrivalTick || before.FulfilledTick != nil || after.FulfilledTick != nil || before.ShipmentID != "" ||
		after.ShipmentID != shipment.ID || shipment.Status != shipmentdomain.StatusInTransit ||
		shipment.SenderHouseholdID != shipmentdomain.HouseholdID(before.DebtorHouseholdID) ||
		shipment.ReceiverHouseholdID != shipmentdomain.HouseholdID(before.CreditorHouseholdID) ||
		shipment.ResourceType != shipmentdomain.ResourceType(before.ResourceType) ||
		shipment.QuantityMilli != shipmentdomain.QuantityMilli(before.QuantityMilli) {
		return shipmentdomain.Shipment{}, ErrContractDispatchStateChanged
	}
	stockTag, err := t.tx.Exec(ctx, `
		UPDATE resource_stocks
		SET quantity_milli = quantity_milli - $3, updated_at = now()
		WHERE household_id = $1::uuid AND resource_code = $2 AND quantity_milli >= $3
	`, before.DebtorHouseholdID, before.ResourceType, before.QuantityMilli)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if stockTag.RowsAffected() != 1 {
		return shipmentdomain.Shipment{}, ErrInsufficientResources
	}
	created, err := t.store.insertShipment(ctx, t.tx, shipment)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	id, err := uuidParam(string(before.ID))
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	shipmentID, err := uuidParam(string(created.ID))
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	rows, err := sqlcdb.New(t.tx).LinkContractObligationShipment(ctx, sqlcdb.LinkContractObligationShipmentParams{
		Column1: id, Column2: shipmentID, Status: string(after.Status), Status_2: string(before.Status),
	})
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if rows != 1 {
		return shipmentdomain.Shipment{}, ErrContractDispatchStateChanged
	}
	for _, fact := range []struct {
		household contractdomain.HouseholdID
		related   contractdomain.HouseholdID
	}{
		{before.DebtorHouseholdID, before.CreditorHouseholdID},
		{before.CreditorHouseholdID, before.DebtorHouseholdID},
	} {
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO chronicle_entries(
				household_id, occurred_tick, entry_type, related_household_id,
				related_contract_id, related_shipment_id, data
			) VALUES (
				$1::uuid, $2, 'contract_shipment_dispatched', $3::uuid,
				$4::uuid, $5::uuid,
				jsonb_build_object('resource_type', $6::text, 'quantity_milli', $7::bigint,
				                   'due_arrival_tick', $8::bigint, 'expected_arrival_tick', $9::bigint)
			)
		`, fact.household, shipment.DepartureTick, fact.related, before.ContractID, created.ID,
			before.ResourceType, before.QuantityMilli, before.DueArrivalTick, shipment.ExpectedArrivalTick); err != nil {
			return shipmentdomain.Shipment{}, err
		}
	}
	return created, nil
}

func nullableContractTick(value *contractdomain.Tick) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: int64(*value), Valid: true}
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
