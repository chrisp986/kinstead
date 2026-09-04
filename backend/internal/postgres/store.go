//go:build postgres

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	contractdomain "game/backend/internal/domain/contract"
	relationshipdomain "game/backend/internal/domain/relationship"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
	sqlcdb "game/backend/internal/postgres/db"
	"game/backend/internal/simulation"
)

type Store struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

func uuidParam(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

type WorldClaim = port.WorldClaim

type worldTickTx struct {
	store *Store
	tx    pgx.Tx
}

func (s *Store) BeginWorldTick(ctx context.Context) (port.WorldTickTransaction, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &worldTickTx{store: s, tx: tx}, nil
}

func (t *worldTickTx) ClaimDueWorld(ctx context.Context) (port.WorldClaim, bool, error) {
	return t.store.ClaimDueWorld(ctx, t.tx)
}
func (t *worldTickTx) IsTickProcessed(ctx context.Context, worldID string, tick int64) (bool, error) {
	return t.store.IsTickProcessed(ctx, t.tx, worldID, tick)
}
func (t *worldTickTx) LoadDueShipments(ctx context.Context, worldID string, tick int64) ([]shipmentdomain.Shipment, error) {
	return t.store.LoadDueShipments(ctx, t.tx, worldID, tick)
}
func (t *worldTickTx) PersistShipmentArrival(ctx context.Context, value shipmentdomain.Shipment) (bool, error) {
	return t.store.PersistShipmentArrival(ctx, t.tx, value)
}
func (t *worldTickTx) LoadContractObligationsForTick(ctx context.Context, worldID string, tick int64) ([]port.ContractObligationAssessment, error) {
	return t.store.LoadContractObligationsForTick(ctx, t.tx, worldID, tick)
}
func (t *worldTickTx) PersistContractObligationAssessment(ctx context.Context, before, after contractdomain.Obligation, event *relationshipdomain.Event) (bool, error) {
	return t.store.PersistContractObligationAssessment(ctx, t.tx, before, after, event)
}
func (t *worldTickTx) LoadActiveContractsForRollup(ctx context.Context, worldID string) ([]port.ContractRollupSnapshot, error) {
	return t.store.LoadActiveContractsForRollup(ctx, t.tx, worldID)
}
func (t *worldTickTx) PersistContractRollup(ctx context.Context, before, after contractdomain.Contract) (bool, error) {
	return t.store.PersistContractRollup(ctx, t.tx, before, after)
}
func (t *worldTickTx) ListHouseholdIDs(ctx context.Context, worldID string) ([]string, error) {
	return t.store.ListHouseholdIDs(ctx, t.tx, worldID)
}
func (t *worldTickTx) LoadHouseholdForTick(ctx context.Context, householdID string, tick int64) (port.HouseholdSnapshot, []simulation.Assignment, error) {
	return t.store.LoadHouseholdForTick(ctx, t.tx, householdID, tick)
}
func (t *worldTickTx) SaveHouseholdTick(ctx context.Context, householdID string, result simulation.TickResult) error {
	return t.store.SaveHouseholdTick(ctx, t.tx, householdID, result)
}
func (t *worldTickTx) ScheduleEmergencyFoodWork(ctx context.Context, householdID, characterID, activity string, startsTick, endsTick int64, supplyDays float64) error {
	return t.store.ScheduleEmergencyFoodWork(ctx, t.tx, householdID, characterID, activity, startsTick, endsTick, supplyDays)
}
func (t *worldTickTx) FinishWorldTick(ctx context.Context, world port.WorldClaim, tick int64) error {
	return t.store.FinishWorldTick(ctx, t.tx, world, tick)
}
func (t *worldTickTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *worldTickTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
}

func (s *Store) ClaimDueWorld(ctx context.Context, tx pgx.Tx) (WorldClaim, bool, error) {
	row, err := sqlcdb.New(tx).ClaimDueWorld(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorldClaim{}, false, nil
	}
	if err != nil {
		return WorldClaim{}, false, err
	}
	if !row.NextTickAt.Valid {
		return WorldClaim{}, false, fmt.Errorf("world %s has invalid next_tick_at", row.ID)
	}
	w := WorldClaim{ID: row.ID, CurrentTick: row.CurrentTick, TickDurationSeconds: row.TickDurationSeconds, NextTickAt: row.NextTickAt.Time}
	return w, true, nil
}

func (s *Store) IsTickProcessed(ctx context.Context, tx pgx.Tx, worldID string, tick int64) (bool, error) {
	id, err := uuidParam(worldID)
	if err != nil {
		return false, err
	}
	return sqlcdb.New(tx).IsWorldTickProcessed(ctx, sqlcdb.IsWorldTickProcessedParams{Column1: id, Tick: tick})
}

func (s *Store) ListHouseholdIDs(ctx context.Context, tx pgx.Tx, worldID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
        SELECT id::text FROM households
        WHERE world_id = $1::uuid
        ORDER BY id
    `, worldID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) LoadDueShipments(ctx context.Context, tx pgx.Tx, worldID string, tick int64) ([]shipmentdomain.Shipment, error) {
	id, err := uuidParam(worldID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcdb.New(tx).LoadShipmentsDueForArrival(ctx, sqlcdb.LoadShipmentsDueForArrivalParams{Column1: id, ExpectedArrivalTick: tick})
	if err != nil {
		return nil, err
	}
	shipments := make([]shipmentdomain.Shipment, 0, len(rows))
	for _, row := range rows {
		value := shipmentFromSQLC(row.ID, row.WorldID, row.SenderHouseholdID, row.ReceiverHouseholdID,
			row.OriginLocationID, row.DestinationLocationID, row.ResourceCode, row.QuantityMilli,
			row.DepartureTick, row.ExpectedArrivalTick, row.ActualArrivalTick, row.TransportCostMilli, row.Status)
		if err := value.Validate(); err != nil {
			return nil, fmt.Errorf("shipment %s: %w", value.ID, err)
		}
		shipments = append(shipments, value)
	}
	return shipments, nil
}

// PersistShipmentArrival credits inventory, marks the shipment arrived, and records
// the structured chronicle fact. The guarded status update makes delivery idempotent.
func (s *Store) PersistShipmentArrival(ctx context.Context, tx pgx.Tx, value shipmentdomain.Shipment) (bool, error) {
	if err := value.Validate(); err != nil {
		return false, err
	}
	if value.Status != shipmentdomain.StatusArrived || value.ActualArrivalTick == nil {
		return false, shipmentdomain.ErrInvalidTransition
	}

	shipmentID, err := uuidParam(string(value.ID))
	if err != nil {
		return false, err
	}
	worldID, err := uuidParam(string(value.WorldID))
	if err != nil {
		return false, err
	}
	_, err = sqlcdb.New(tx).MarkShipmentArrived(ctx, sqlcdb.MarkShipmentArrivedParams{
		Column1: shipmentID, ActualArrivalTick: pgtype.Int8{Int64: int64(*value.ActualArrivalTick), Valid: true}, Column3: worldID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO resource_stocks(household_id, resource_code, quantity_milli, updated_at)
        VALUES ($1::uuid, $2, $3, now())
        ON CONFLICT (household_id, resource_code)
        DO UPDATE SET quantity_milli = resource_stocks.quantity_milli + EXCLUDED.quantity_milli,
                      updated_at = now()
    `, value.ReceiverHouseholdID, value.ResourceType, value.QuantityMilli); err != nil {
		return false, err
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO chronicle_entries(
            household_id, occurred_tick, entry_type, related_household_id,
            related_shipment_id, data
        ) VALUES (
            $1::uuid, $2, 'shipment_arrived', $3::uuid, $4::uuid,
            jsonb_build_object('resource_type', $5::text, 'quantity_milli', $6::bigint)
        )
    `, value.ReceiverHouseholdID, *value.ActualArrivalTick, value.SenderHouseholdID,
		value.ID, value.ResourceType, value.QuantityMilli); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) CreateShipment(ctx context.Context, value shipmentdomain.Shipment) (shipmentdomain.Shipment, error) {
	if err := value.Validate(); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if value.Status != shipmentdomain.StatusInTransit || value.ActualArrivalTick != nil {
		return shipmentdomain.Shipment{}, shipmentdomain.ErrInvalidTransition
	}

	tx, err := s.Begin(ctx)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	defer tx.Rollback(ctx)

	var currentTick int64
	err = tx.QueryRow(ctx, `
        SELECT w.current_tick
        FROM worlds w
        JOIN households sender
          ON sender.id = $2::uuid AND sender.world_id = w.id AND sender.location_id = $4::uuid
        JOIN households receiver
          ON receiver.id = $3::uuid AND receiver.world_id = w.id AND receiver.location_id = $5::uuid
        JOIN locations origin ON origin.id = $4::uuid AND origin.world_id = w.id
        JOIN locations destination ON destination.id = $5::uuid AND destination.world_id = w.id
        JOIN resource_types resource ON resource.code = $6
        WHERE w.id = $1::uuid
        FOR UPDATE OF w, sender, receiver
    `, value.WorldID, value.SenderHouseholdID, value.ReceiverHouseholdID,
		value.OriginLocationID, value.DestinationLocationID, value.ResourceType).Scan(&currentTick)
	if errors.Is(err, pgx.ErrNoRows) {
		return shipmentdomain.Shipment{}, ErrInvalidShipmentReferences
	}
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if shipmentdomain.Tick(currentTick) != value.DepartureTick {
		return shipmentdomain.Shipment{}, ErrShipmentTickConflict
	}

	tag, err := tx.Exec(ctx, `
        UPDATE resource_stocks
        SET quantity_milli = quantity_milli - $3, updated_at = now()
        WHERE household_id = $1::uuid AND resource_code = $2 AND quantity_milli >= $3
    `, value.SenderHouseholdID, value.ResourceType, value.QuantityMilli)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if tag.RowsAffected() != 1 {
		return shipmentdomain.Shipment{}, ErrInsufficientResources
	}

	created, err := s.insertShipment(ctx, tx, value)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	return created, nil
}

func (s *Store) insertShipment(ctx context.Context, tx pgx.Tx, value shipmentdomain.Shipment) (shipmentdomain.Shipment, error) {
	return scanShipment(tx.QueryRow(ctx, `
        INSERT INTO shipments(
            id, world_id, sender_household_id, receiver_household_id,
            origin_location_id, destination_location_id, resource_code,
            quantity_milli, departure_tick, expected_arrival_tick,
            transport_cost_milli, status
        ) VALUES (
            COALESCE(NULLIF($1::text, '')::uuid, gen_random_uuid()), $2::uuid, $3::uuid, $4::uuid,
            $5::uuid, $6::uuid, $7, $8, $9, $10, $11, 'in_transit'
        )
        RETURNING id::text, world_id::text, sender_household_id::text, receiver_household_id::text,
                  origin_location_id::text, destination_location_id::text, resource_code,
                  quantity_milli, departure_tick, expected_arrival_tick,
                  actual_arrival_tick, transport_cost_milli, status
    `, value.ID, value.WorldID, value.SenderHouseholdID, value.ReceiverHouseholdID,
		value.OriginLocationID, value.DestinationLocationID, value.ResourceType,
		value.QuantityMilli, value.DepartureTick, value.ExpectedArrivalTick, value.TransportCostMilli))
}

func (s *Store) ListHouseholdShipments(ctx context.Context, householdID string) ([]ShipmentRecord, error) {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM households WHERE id = $1::uuid)`, householdID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, pgx.ErrNoRows
	}

	id, err := uuidParam(householdID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcdb.New(s.Pool).ListShipmentsByHousehold(ctx, id)
	if err != nil {
		return nil, err
	}
	records := make([]ShipmentRecord, 0, len(rows))
	for _, row := range rows {
		value := shipmentFromSQLC(row.ID, row.WorldID, row.SenderHouseholdID, row.ReceiverHouseholdID,
			row.OriginLocationID, row.DestinationLocationID, row.ResourceCode, row.QuantityMilli,
			row.DepartureTick, row.ExpectedArrivalTick, row.ActualArrivalTick, row.TransportCostMilli, row.Status)
		records = append(records, shipmentRecord(value))
	}
	return records, nil
}

// CancelShipment refunds a reserved direct transfer exactly once. Market
// shipments are final sale settlements and require a future explicit reversal
// workflow rather than this direct-shipment command.
func (s *Store) CancelShipment(ctx context.Context, shipmentID shipmentdomain.ID, senderID shipmentdomain.HouseholdID) (shipmentdomain.Shipment, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	defer tx.Rollback(ctx)

	var currentTick int64
	value, err := scanShipment(tx.QueryRow(ctx, `
		SELECT s.id::text, s.world_id::text, s.sender_household_id::text, s.receiver_household_id::text,
		       s.origin_location_id::text, s.destination_location_id::text, s.resource_code,
		       s.quantity_milli, s.departure_tick, s.expected_arrival_tick,
		       s.actual_arrival_tick, s.transport_cost_milli, s.status
		FROM shipments s
		JOIN worlds w ON w.id = s.world_id
		WHERE s.id = $1::uuid
		FOR UPDATE OF s, w
	`, shipmentID))
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT current_tick FROM worlds WHERE id = $1::uuid`, value.WorldID).Scan(&currentTick); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if value.SenderHouseholdID != senderID {
		return shipmentdomain.Shipment{}, shipmentdomain.ErrCancellationForbidden
	}
	var marketCreated bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM chronicle_entries
			WHERE related_shipment_id = $1::uuid
			  AND entry_type IN ('market_purchase', 'market_sale')
		)
	`, shipmentID).Scan(&marketCreated); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if marketCreated {
		return shipmentdomain.Shipment{}, shipmentdomain.ErrCancellationForbidden
	}
	if value.Status == shipmentdomain.StatusCancelled {
		// A retry after a committed cancellation is a successful no-op. The
		// guarded mutation below remains the only path that refunds inventory.
		return value, nil
	}
	if value.Status != shipmentdomain.StatusInTransit {
		return shipmentdomain.Shipment{}, shipmentdomain.ErrCancellationClosed
	}
	cancelled, err := value.CancelAt(shipmentdomain.Tick(currentTick))
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE shipments SET status = 'cancelled'
		WHERE id = $1::uuid AND status = 'in_transit'
	`, shipmentID)
	if err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if tag.RowsAffected() != 1 {
		return shipmentdomain.Shipment{}, shipmentdomain.ErrCancellationClosed
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO resource_stocks(household_id, resource_code, quantity_milli, updated_at)
		VALUES ($1::uuid, $2, $3, now())
		ON CONFLICT (household_id, resource_code)
		DO UPDATE SET quantity_milli = resource_stocks.quantity_milli + EXCLUDED.quantity_milli,
		              updated_at = now()
	`, cancelled.SenderHouseholdID, cancelled.ResourceType, cancelled.QuantityMilli); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO chronicle_entries(
			household_id, occurred_tick, entry_type, related_household_id, related_shipment_id, data
		) VALUES (
			$1::uuid, $2, 'shipment_cancelled', $3::uuid, $4::uuid,
			jsonb_build_object('resource_type', $5::text, 'quantity_milli', $6::bigint)
		)
	`, cancelled.SenderHouseholdID, currentTick, cancelled.ReceiverHouseholdID,
		cancelled.ID, cancelled.ResourceType, cancelled.QuantityMilli); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	return cancelled, nil
}

type CharacterRecord = port.CharacterRecord
type AssignmentRecord = port.AssignmentRecord
type ShipmentRecord = port.ShipmentRecord

var (
	ErrInvalidShipmentReferences = errors.New("invalid shipment references")
	ErrInsufficientResources     = errors.New("insufficient resources for shipment")
	ErrShipmentTickConflict      = errors.New("shipment departure tick is not current")
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanShipment(row rowScanner) (shipmentdomain.Shipment, error) {
	var record ShipmentRecord
	if err := row.Scan(
		&record.ID, &record.WorldID, &record.SenderHouseholdID, &record.ReceiverHouseholdID,
		&record.OriginLocationID, &record.DestinationLocationID, &record.ResourceType,
		&record.QuantityMilli, &record.DepartureTick, &record.ExpectedArrivalTick,
		&record.ActualArrivalTick, &record.TransportCostMilli, &record.Status,
	); err != nil {
		return shipmentdomain.Shipment{}, err
	}
	var actual *shipmentdomain.Tick
	if record.ActualArrivalTick != nil {
		v := shipmentdomain.Tick(*record.ActualArrivalTick)
		actual = &v
	}
	return shipmentdomain.Shipment{
		ID: shipmentdomain.ID(record.ID), WorldID: shipmentdomain.WorldID(record.WorldID),
		SenderHouseholdID:     shipmentdomain.HouseholdID(record.SenderHouseholdID),
		ReceiverHouseholdID:   shipmentdomain.HouseholdID(record.ReceiverHouseholdID),
		OriginLocationID:      shipmentdomain.LocationID(record.OriginLocationID),
		DestinationLocationID: shipmentdomain.LocationID(record.DestinationLocationID),
		ResourceType:          shipmentdomain.ResourceType(record.ResourceType),
		QuantityMilli:         shipmentdomain.QuantityMilli(record.QuantityMilli),
		DepartureTick:         shipmentdomain.Tick(record.DepartureTick),
		ExpectedArrivalTick:   shipmentdomain.Tick(record.ExpectedArrivalTick),
		ActualArrivalTick:     actual, TransportCostMilli: shipmentdomain.MoneyMilli(record.TransportCostMilli),
		Status: shipmentdomain.Status(record.Status),
	}, nil
}

func shipmentFromSQLC(id, worldID, senderID, receiverID, originID, destinationID, resource string,
	quantity, departure, expected int64, actualValue pgtype.Int8, transport int64, status string,
) shipmentdomain.Shipment {
	var actual *shipmentdomain.Tick
	if actualValue.Valid {
		value := shipmentdomain.Tick(actualValue.Int64)
		actual = &value
	}
	return shipmentdomain.Shipment{
		ID: shipmentdomain.ID(id), WorldID: shipmentdomain.WorldID(worldID),
		SenderHouseholdID: shipmentdomain.HouseholdID(senderID), ReceiverHouseholdID: shipmentdomain.HouseholdID(receiverID),
		OriginLocationID: shipmentdomain.LocationID(originID), DestinationLocationID: shipmentdomain.LocationID(destinationID),
		ResourceType: shipmentdomain.ResourceType(resource), QuantityMilli: shipmentdomain.QuantityMilli(quantity),
		DepartureTick: shipmentdomain.Tick(departure), ExpectedArrivalTick: shipmentdomain.Tick(expected),
		ActualArrivalTick: actual, TransportCostMilli: shipmentdomain.MoneyMilli(transport), Status: shipmentdomain.Status(status),
	}
}

func shipmentRecord(value shipmentdomain.Shipment) ShipmentRecord {
	var actual *int64
	if value.ActualArrivalTick != nil {
		v := int64(*value.ActualArrivalTick)
		actual = &v
	}
	return ShipmentRecord{
		ID: string(value.ID), WorldID: string(value.WorldID),
		SenderHouseholdID: string(value.SenderHouseholdID), ReceiverHouseholdID: string(value.ReceiverHouseholdID),
		OriginLocationID: string(value.OriginLocationID), DestinationLocationID: string(value.DestinationLocationID),
		ResourceType: string(value.ResourceType), QuantityMilli: int64(value.QuantityMilli),
		DepartureTick: int64(value.DepartureTick), ExpectedArrivalTick: int64(value.ExpectedArrivalTick),
		ActualArrivalTick: actual, TransportCostMilli: int64(value.TransportCostMilli), Status: string(value.Status),
	}
}

type HouseholdSnapshot = port.HouseholdSnapshot

func (s *Store) LoadHouseholdForTick(ctx context.Context, tx pgx.Tx, householdID string, tick int64) (HouseholdSnapshot, []simulation.Assignment, error) {
	var snap HouseholdSnapshot
	err := tx.QueryRow(ctx, `
        SELECT h.id::text, h.name, h.world_id::text, w.name, w.current_tick,
               w.historical_start_date::timestamp, w.historical_days_per_tick_num, w.historical_days_per_tick_den,
               w.tick_duration_seconds, COALESCE(h.specialization, '')
        FROM households h
        JOIN worlds w ON w.id = h.world_id
        WHERE h.id = $1::uuid
        FOR UPDATE OF h, w
    `, householdID).Scan(
		&snap.HouseholdID, &snap.HouseholdName, &snap.WorldID, &snap.WorldName,
		&snap.CurrentTick, &snap.HistoricalStart, &snap.HistoricalDaysPerTickNum, &snap.HistoricalDaysPerTickDen, &snap.TickDurationSeconds, &snap.Specialization,
	)
	if err != nil {
		return snap, nil, err
	}

	stocks := map[string]int64{}
	rows, err := tx.Query(ctx, `
        SELECT resource_code, quantity_milli
        FROM resource_stocks
        WHERE household_id = $1::uuid
        FOR UPDATE
    `, householdID)
	if err != nil {
		return snap, nil, err
	}
	for rows.Next() {
		var code string
		var q int64
		if err := rows.Scan(&code, &q); err != nil {
			rows.Close()
			return snap, nil, err
		}
		stocks[code] = q
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, nil, err
	}

	charRows, err := tx.Query(ctx, `
		SELECT c.id::text, c.name, c.birth_date::text, c.labor_capacity_milli, c.fatigue,
               COALESCE((
                   SELECT cs.skill_code FROM character_skills cs
                   WHERE cs.character_id = c.id AND cs.level > 0
                   ORDER BY cs.level DESC, cs.skill_code
                   LIMIT 1
               ), '') AS specialization
        FROM characters c
        WHERE c.household_id = $1::uuid AND c.status = 'active'
        ORDER BY c.created_at, c.id
        FOR UPDATE OF c
    `, householdID)
	if err != nil {
		return snap, nil, err
	}
	var chars []simulation.Character
	for charRows.Next() {
		var cr CharacterRecord
		if err := charRows.Scan(&cr.ID, &cr.Name, &cr.BirthDate, &cr.LaborPermille, &cr.Fatigue, &cr.Specialization); err != nil {
			charRows.Close()
			return snap, nil, err
		}
		snap.Characters = append(snap.Characters, cr)
		chars = append(chars, simulation.Character{
			ID: cr.ID, Name: cr.Name, LaborPermille: cr.LaborPermille,
			Fatigue: cr.Fatigue, Specialization: simulation.Activity(cr.Specialization),
		})
	}
	charRows.Close()
	if err := charRows.Err(); err != nil {
		return snap, nil, err
	}

	aRows, err := tx.Query(ctx, `
        SELECT a.id::text, a.character_id::text, c.name, a.activity_type,
               a.intensity, a.starts_tick, a.ends_tick, a.status
        FROM assignments a
        JOIN characters c ON c.id = a.character_id
        WHERE a.household_id = $1::uuid
          AND a.status IN ('planned','active')
          AND a.starts_tick <= $2
          AND a.ends_tick >= $2
        ORDER BY a.created_at, a.id
        FOR UPDATE OF a
    `, householdID, tick)
	if err != nil {
		return snap, nil, err
	}
	var assignments []simulation.Assignment
	for aRows.Next() {
		var ar AssignmentRecord
		if err := aRows.Scan(&ar.ID, &ar.CharacterID, &ar.Character, &ar.Activity, &ar.Intensity, &ar.StartsTick, &ar.EndsTick, &ar.Status); err != nil {
			aRows.Close()
			return snap, nil, err
		}
		snap.Assignments = append(snap.Assignments, ar)
		assignments = append(assignments, simulation.Assignment{
			Character: ar.Character,
			Activity:  simulation.Activity(ar.Activity),
			Intensity: simulation.Intensity(ar.Intensity),
		})
	}
	aRows.Close()
	if err := aRows.Err(); err != nil {
		return snap, nil, err
	}

	snap.State = simulation.HouseholdState{
		Tick:               snap.CurrentTick,
		FarmSpecialization: dbFarmSpecialization(snap.Specialization),
		ProvisionsMilli:    stocks["provisions"],
		WoodMilli:          stocks["wood"],
		TradeGoodsMilli:    stocks["trade_goods"],
		SilverMilli:        stocks["silver"],
		Characters:         chars,
	}
	return snap, assignments, nil
}

func dbFarmSpecialization(v string) simulation.Activity {
	switch v {
	case "agriculture":
		return simulation.Agriculture
	case "fishing":
		return simulation.Fishing
	case "forest":
		return simulation.Woodcutting
	default:
		return ""
	}
}

func (s *Store) SaveHouseholdTick(ctx context.Context, tx pgx.Tx, householdID string, result simulation.TickResult) error {
	stocks := map[string]int64{
		"provisions":  result.State.ProvisionsMilli,
		"wood":        result.State.WoodMilli,
		"trade_goods": result.State.TradeGoodsMilli,
		"silver":      result.State.SilverMilli,
	}
	for code, quantity := range stocks {
		if _, err := tx.Exec(ctx, `
            INSERT INTO resource_stocks(household_id, resource_code, quantity_milli, updated_at)
            VALUES ($1::uuid, $2, $3, now())
            ON CONFLICT (household_id, resource_code)
            DO UPDATE SET quantity_milli = EXCLUDED.quantity_milli, updated_at = now()
        `, householdID, code, quantity); err != nil {
			return err
		}
	}

	for _, c := range result.State.Characters {
		if c.ID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE characters SET fatigue = $2, updated_at = now() WHERE id = $1::uuid`, c.ID, c.Fatigue); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO chronicle_entries(
            household_id, occurred_tick, entry_type, subject_character_id,
            related_assignment_id, data
        )
        SELECT a.household_id, $2, 'assignment_completed', a.character_id, a.id,
               jsonb_build_object(
                   'activity', a.activity_type,
                   'intensity', a.intensity,
                   'starts_tick', a.starts_tick,
                   'ends_tick', a.ends_tick
               )
        FROM assignments a
        WHERE a.household_id = $1::uuid
          AND a.status IN ('planned','active')
          AND a.ends_tick <= $2
        ON CONFLICT DO NOTHING
    `, householdID, result.State.Tick); err != nil {
		return err
	}

	_, err := tx.Exec(ctx, `
        UPDATE assignments
        SET status = CASE
            WHEN ends_tick <= $2 THEN 'completed'
            WHEN starts_tick <= $2 THEN 'active'
            ELSE status END,
            updated_at = now()
        WHERE household_id = $1::uuid
          AND status IN ('planned','active')
    `, householdID, result.State.Tick)
	return err
}

func (s *Store) ScheduleEmergencyFoodWork(ctx context.Context, tx pgx.Tx, householdID, characterID, activity string, startsTick, endsTick int64, supplyDays float64) error {
	if supplyDays >= 7 || (activity != string(simulation.Agriculture) && activity != string(simulation.Fishing)) || startsTick != endsTick {
		return nil
	}
	var eligible bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM characters c
			WHERE c.id=$1::uuid AND c.household_id=$2::uuid
			  AND c.status='active' AND c.labor_capacity_milli=1000
		)`, characterID, householdID).Scan(&eligible); err != nil {
		return err
	}
	if !eligible {
		return nil
	}
	var overlap bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM assignments WHERE character_id=$1::uuid
		  AND status IN ('planned','active') AND starts_tick <= $3 AND ends_tick >= $2
	)`, characterID, startsTick, endsTick).Scan(&overlap); err != nil {
		return err
	}
	if overlap {
		return nil
	}
	var assignmentID string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE((SELECT id::text FROM assignments
		 WHERE character_id=$1::uuid AND starts_tick=$2 AND ends_tick=$2
		   AND status IN ('planned','active') AND metadata->>'source'='emergency'
		 ORDER BY id LIMIT 1), '')`, characterID, startsTick).Scan(&assignmentID); err != nil {
		return err
	}
	if assignmentID != "" {
		return nil
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO assignments(household_id, character_id, activity_type, intensity, starts_tick, ends_tick, status, metadata)
		VALUES ($1::uuid,$2::uuid,$3,'normal',$4,$5,'planned',jsonb_build_object('source','emergency','reason','supply_emergency','created_tick',$6))
		RETURNING id::text`, householdID, characterID, activity, startsTick, endsTick, startsTick-1).Scan(&assignmentID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO chronicle_entries(household_id, occurred_tick, entry_type, subject_character_id, related_assignment_id, data)
		VALUES ($1::uuid,$2,'emergency_food_work_scheduled',$3::uuid,$4::uuid,
			jsonb_build_object('character_id',$3::text,'activity',$5::text,'starts_tick',$6::bigint,'ends_tick',$7::bigint,'reason','supply_emergency','supply_days',$8::numeric))
		ON CONFLICT (related_assignment_id, entry_type) DO NOTHING`, householdID, startsTick-1, characterID, assignmentID, activity, startsTick, endsTick, supplyDays)
	return err
}

func (s *Store) FinishWorldTick(ctx context.Context, tx pgx.Tx, world WorldClaim, tick int64) error {
	worldID, err := uuidParam(world.ID)
	if err != nil {
		return err
	}
	if err := sqlcdb.New(tx).MarkWorldTickProcessed(ctx, sqlcdb.MarkWorldTickProcessedParams{Column1: worldID, Tick: tick}); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
        UPDATE worlds
        SET current_tick = $2,
            next_tick_at = next_tick_at + (tick_duration_seconds * interval '1 second'),
            updated_at = now()
        WHERE id = $1::uuid AND current_tick = $3
    `, world.ID, tick, world.CurrentTick)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("world tick concurrent update detected")
	}
	return nil
}

func (s *Store) GetHouseholdReport(ctx context.Context, householdID string) (HouseholdSnapshot, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return HouseholdSnapshot{}, err
	}
	defer tx.Rollback(ctx)

	var currentTick int64
	if err := tx.QueryRow(ctx, `
        SELECT w.current_tick FROM households h JOIN worlds w ON w.id=h.world_id WHERE h.id=$1::uuid
    `, householdID).Scan(&currentTick); err != nil {
		return HouseholdSnapshot{}, err
	}

	snap, _, err := s.LoadHouseholdReadOnly(ctx, tx, householdID, currentTick)
	if err != nil {
		return HouseholdSnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return HouseholdSnapshot{}, err
	}
	return snap, nil
}

func (s *Store) LoadHouseholdReadOnly(ctx context.Context, tx pgx.Tx, householdID string, tick int64) (HouseholdSnapshot, []simulation.Assignment, error) {
	// Read-only twin of LoadHouseholdForTick. It intentionally avoids row locks for API projections.
	var snap HouseholdSnapshot
	err := tx.QueryRow(ctx, `
        SELECT h.id::text, h.name, h.world_id::text, w.name, w.current_tick,
               w.historical_start_date::timestamp, w.historical_days_per_tick_num, w.historical_days_per_tick_den,
               w.tick_duration_seconds, COALESCE(h.specialization, '')
        FROM households h JOIN worlds w ON w.id=h.world_id
        WHERE h.id=$1::uuid
    `, householdID).Scan(&snap.HouseholdID, &snap.HouseholdName, &snap.WorldID, &snap.WorldName, &snap.CurrentTick, &snap.HistoricalStart, &snap.HistoricalDaysPerTickNum, &snap.HistoricalDaysPerTickDen, &snap.TickDurationSeconds, &snap.Specialization)
	if err != nil {
		return snap, nil, err
	}

	stocks := map[string]int64{}
	rows, err := tx.Query(ctx, `SELECT resource_code, quantity_milli FROM resource_stocks WHERE household_id=$1::uuid`, householdID)
	if err != nil {
		return snap, nil, err
	}
	for rows.Next() {
		var c string
		var q int64
		if err := rows.Scan(&c, &q); err != nil {
			rows.Close()
			return snap, nil, err
		}
		stocks[c] = q
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, nil, err
	}

	rows, err = tx.Query(ctx, `
		SELECT c.id::text,c.name,c.birth_date::text,c.labor_capacity_milli,c.fatigue,
        COALESCE((SELECT cs.skill_code FROM character_skills cs WHERE cs.character_id=c.id AND cs.level>0 ORDER BY cs.level DESC,cs.skill_code LIMIT 1),'')
        FROM characters c WHERE c.household_id=$1::uuid AND c.status='active' ORDER BY c.created_at,c.id
    `, householdID)
	if err != nil {
		return snap, nil, err
	}
	var chars []simulation.Character
	for rows.Next() {
		var cr CharacterRecord
		if err := rows.Scan(&cr.ID, &cr.Name, &cr.BirthDate, &cr.LaborPermille, &cr.Fatigue, &cr.Specialization); err != nil {
			rows.Close()
			return snap, nil, err
		}
		snap.Characters = append(snap.Characters, cr)
		chars = append(chars, simulation.Character{ID: cr.ID, Name: cr.Name, LaborPermille: cr.LaborPermille, Fatigue: cr.Fatigue, Specialization: simulation.Activity(cr.Specialization)})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, nil, err
	}

	rows, err = tx.Query(ctx, `
        SELECT a.id::text,a.character_id::text,c.name,a.activity_type,a.intensity,a.starts_tick,a.ends_tick,a.status
        FROM assignments a JOIN characters c ON c.id=a.character_id
        WHERE a.household_id=$1::uuid AND a.status IN ('planned','active') AND a.ends_tick >= $2
        ORDER BY a.starts_tick,a.created_at
    `, householdID, tick)
	if err != nil {
		return snap, nil, err
	}
	var assignments []simulation.Assignment
	for rows.Next() {
		var ar AssignmentRecord
		if err := rows.Scan(&ar.ID, &ar.CharacterID, &ar.Character, &ar.Activity, &ar.Intensity, &ar.StartsTick, &ar.EndsTick, &ar.Status); err != nil {
			rows.Close()
			return snap, nil, err
		}
		snap.Assignments = append(snap.Assignments, ar)
		if ar.StartsTick <= tick && ar.EndsTick >= tick {
			assignments = append(assignments, simulation.Assignment{Character: ar.Character, Activity: simulation.Activity(ar.Activity), Intensity: simulation.Intensity(ar.Intensity)})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, nil, err
	}

	snap.State = simulation.HouseholdState{Tick: snap.CurrentTick, FarmSpecialization: dbFarmSpecialization(snap.Specialization), ProvisionsMilli: stocks["provisions"], WoodMilli: stocks["wood"], TradeGoodsMilli: stocks["trade_goods"], SilverMilli: stocks["silver"], Characters: chars}
	return snap, assignments, nil
}

func (s *Store) CreateAssignment(ctx context.Context, householdID, characterID, activity, intensity string, startsTick, endsTick int64) (AssignmentRecord, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return AssignmentRecord{}, err
	}
	defer tx.Rollback(ctx)

	var currentTick int64
	var characterName string
	// Lock the world row so worker ticks and work-plan writes serialize cleanly.
	err = tx.QueryRow(ctx, `
        SELECT w.current_tick, c.name
        FROM households h
        JOIN worlds w ON w.id=h.world_id
        JOIN characters c ON c.household_id=h.id AND c.id=$2::uuid
        WHERE h.id=$1::uuid
        FOR UPDATE OF w
    `, householdID, characterID).Scan(&currentTick, &characterName)
	if err != nil {
		return AssignmentRecord{}, err
	}
	if startsTick <= currentTick {
		return AssignmentRecord{}, fmt.Errorf("starts_tick must be greater than current tick %d", currentTick)
	}

	type emergencyOverlap struct{ id, activity string }
	emergency := make([]emergencyOverlap, 0)
	rows, err := tx.Query(ctx, `
		SELECT id::text, activity_type, starts_tick, metadata->>'source'
		FROM assignments
		WHERE character_id=$1::uuid AND status IN ('planned','active')
		  AND starts_tick <= $3 AND ends_tick >= $2`, characterID, startsTick, endsTick)
	if err != nil {
		return AssignmentRecord{}, err
	}
	blocking := false
	for rows.Next() {
		var id, activity, source string
		var existingStart int64
		if err := rows.Scan(&id, &activity, &existingStart, &source); err != nil {
			rows.Close()
			return AssignmentRecord{}, err
		}
		if source == "emergency" && existingStart > currentTick {
			emergency = append(emergency, emergencyOverlap{id: id, activity: activity})
		} else {
			blocking = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return AssignmentRecord{}, err
	}
	if blocking {
		return AssignmentRecord{}, fmt.Errorf("assignment overlaps existing work plan")
	}
	for _, value := range emergency {
		if _, err := tx.Exec(ctx, `DELETE FROM assignments WHERE id=$1::uuid AND status='planned'`, value.id); err != nil {
			return AssignmentRecord{}, err
		}
	}

	var out AssignmentRecord
	err = tx.QueryRow(ctx, `
        INSERT INTO assignments(household_id, character_id, activity_type, intensity, starts_tick, ends_tick, status)
        VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,'planned')
        RETURNING id::text, character_id::text, $7::text, activity_type, intensity, starts_tick, ends_tick, status
    `, householdID, characterID, activity, intensity, startsTick, endsTick, characterName).Scan(
		&out.ID, &out.CharacterID, &out.Character, &out.Activity, &out.Intensity, &out.StartsTick, &out.EndsTick, &out.Status,
	)
	if err != nil {
		return AssignmentRecord{}, err
	}
	if _, err := tx.Exec(ctx, `
        INSERT INTO chronicle_entries(
            household_id, occurred_tick, entry_type, subject_character_id,
            related_assignment_id, data
        ) VALUES (
            $1::uuid, $2, 'assignment_scheduled', $3::uuid, $4::uuid,
            jsonb_build_object(
                'activity', $5::text,
                'intensity', $6::text,
                'starts_tick', $7::bigint,
                'ends_tick', $8::bigint
            )
        )
	`, householdID, currentTick, characterID, out.ID, activity, intensity, startsTick, endsTick); err != nil {
		return AssignmentRecord{}, err
	}
	for _, value := range emergency {
		if _, err := tx.Exec(ctx, `
			INSERT INTO chronicle_entries(household_id, occurred_tick, entry_type, subject_character_id, data)
			VALUES ($1::uuid,$2,'emergency_work_overridden',$3::uuid,
				jsonb_build_object('character_id',$3::text,'emergency_activity',$4::text,'replacement_activity',$5::text,'starts_tick',$6::bigint))
			ON CONFLICT DO NOTHING`, householdID, currentTick, characterID, value.activity, activity, startsTick); err != nil {
			return AssignmentRecord{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return AssignmentRecord{}, err
	}
	return out, nil
}
