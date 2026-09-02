//go:build postgres

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	shipmentdomain "game/backend/internal/domain/shipment"
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

type WorldClaim struct {
	ID                  string
	CurrentTick         int64
	TickDurationSeconds int32
	NextTickAt          time.Time
}

func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
}

func (s *Store) ClaimDueWorld(ctx context.Context, tx pgx.Tx) (WorldClaim, bool, error) {
	var w WorldClaim
	err := tx.QueryRow(ctx, `
        SELECT id::text, current_tick, tick_duration_seconds, next_tick_at
        FROM worlds
        WHERE next_tick_at <= now()
        ORDER BY next_tick_at
        FOR UPDATE SKIP LOCKED
        LIMIT 1
    `).Scan(&w.ID, &w.CurrentTick, &w.TickDurationSeconds, &w.NextTickAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorldClaim{}, false, nil
	}
	return w, err == nil, err
}

func (s *Store) IsTickProcessed(ctx context.Context, tx pgx.Tx, worldID string, tick int64) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
        SELECT EXISTS(
            SELECT 1 FROM processed_world_ticks
            WHERE world_id = $1::uuid AND tick = $2
        )
    `, worldID, tick).Scan(&exists)
	return exists, err
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
	rows, err := tx.Query(ctx, `
        SELECT id::text, world_id::text, sender_household_id::text, receiver_household_id::text,
               origin_location_id::text, destination_location_id::text, resource_code,
               quantity_milli, departure_tick, expected_arrival_tick,
               actual_arrival_tick, transport_cost_milli, status
        FROM shipments
        WHERE world_id = $1::uuid
          AND status = 'in_transit'
          AND actual_arrival_tick IS NULL
          AND expected_arrival_tick <= $2
        ORDER BY expected_arrival_tick, id
        FOR UPDATE
    `, worldID, tick)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shipments := make([]shipmentdomain.Shipment, 0)
	for rows.Next() {
		value, err := scanShipment(rows)
		if err != nil {
			return nil, err
		}
		if err := value.Validate(); err != nil {
			return nil, fmt.Errorf("shipment %s: %w", value.ID, err)
		}
		shipments = append(shipments, value)
	}
	return shipments, rows.Err()
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

	tag, err := tx.Exec(ctx, `
        UPDATE shipments
        SET status = 'arrived', actual_arrival_tick = $2
        WHERE id = $1::uuid
          AND world_id = $3::uuid
          AND status = 'in_transit'
          AND actual_arrival_tick IS NULL
          AND expected_arrival_tick <= $2
    `, value.ID, *value.ActualArrivalTick, value.WorldID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
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

	rows, err := s.Pool.Query(ctx, `
        SELECT id::text, world_id::text, sender_household_id::text, receiver_household_id::text,
               origin_location_id::text, destination_location_id::text, resource_code,
               quantity_milli, departure_tick, expected_arrival_tick,
               actual_arrival_tick, transport_cost_milli, status
        FROM shipments
        WHERE sender_household_id = $1::uuid OR receiver_household_id = $1::uuid
        ORDER BY departure_tick DESC, id
    `, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]ShipmentRecord, 0)
	for rows.Next() {
		value, err := scanShipment(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, shipmentRecord(value))
	}
	return records, rows.Err()
}

type CharacterRecord struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LaborPermille  int64  `json:"labor_permille"`
	Fatigue        int    `json:"fatigue"`
	Specialization string `json:"specialization,omitempty"`
}

type AssignmentRecord struct {
	ID          string `json:"id"`
	CharacterID string `json:"character_id"`
	Character   string `json:"character"`
	Activity    string `json:"activity"`
	Intensity   string `json:"intensity"`
	StartsTick  int64  `json:"starts_tick"`
	EndsTick    int64  `json:"ends_tick"`
	Status      string `json:"status"`
}

type ShipmentRecord struct {
	ID                    string `json:"id"`
	WorldID               string `json:"world_id"`
	SenderHouseholdID     string `json:"sender_household_id"`
	ReceiverHouseholdID   string `json:"receiver_household_id"`
	OriginLocationID      string `json:"origin_location_id"`
	DestinationLocationID string `json:"destination_location_id"`
	ResourceType          string `json:"resource_type"`
	QuantityMilli         int64  `json:"quantity_milli"`
	DepartureTick         int64  `json:"departure_tick"`
	ExpectedArrivalTick   int64  `json:"expected_arrival_tick"`
	ActualArrivalTick     *int64 `json:"actual_arrival_tick,omitempty"`
	TransportCostMilli    int64  `json:"transport_cost_milli"`
	Status                string `json:"status"`
}

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

type HouseholdSnapshot struct {
	HouseholdID              string
	HouseholdName            string
	WorldID                  string
	WorldName                string
	CurrentTick              int64
	HistoricalStart          time.Time
	HistoricalDaysPerTickNum int32
	HistoricalDaysPerTickDen int32
	Specialization           string
	State                    simulation.HouseholdState
	Characters               []CharacterRecord
	Assignments              []AssignmentRecord
}

func (s *Store) LoadHouseholdForTick(ctx context.Context, tx pgx.Tx, householdID string, tick int64) (HouseholdSnapshot, []simulation.Assignment, error) {
	var snap HouseholdSnapshot
	err := tx.QueryRow(ctx, `
        SELECT h.id::text, h.name, h.world_id::text, w.name, w.current_tick,
               w.historical_start_date::timestamp, w.historical_days_per_tick_num, w.historical_days_per_tick_den, COALESCE(h.specialization, '')
        FROM households h
        JOIN worlds w ON w.id = h.world_id
        WHERE h.id = $1::uuid
        FOR UPDATE OF h, w
    `, householdID).Scan(
		&snap.HouseholdID, &snap.HouseholdName, &snap.WorldID, &snap.WorldName,
		&snap.CurrentTick, &snap.HistoricalStart, &snap.HistoricalDaysPerTickNum, &snap.HistoricalDaysPerTickDen, &snap.Specialization,
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
        SELECT c.id::text, c.name, c.labor_capacity_milli, c.fatigue,
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
		if err := charRows.Scan(&cr.ID, &cr.Name, &cr.LaborPermille, &cr.Fatigue, &cr.Specialization); err != nil {
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

func (s *Store) FinishWorldTick(ctx context.Context, tx pgx.Tx, world WorldClaim, tick int64) error {
	if _, err := tx.Exec(ctx, `
        INSERT INTO processed_world_ticks(world_id, tick)
        VALUES ($1::uuid, $2)
        ON CONFLICT DO NOTHING
    `, world.ID, tick); err != nil {
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

func (s *Store) GetHouseholdReport(ctx context.Context, householdID string, cfg simulation.BalanceConfig) (HouseholdSnapshot, error) {
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
               w.historical_start_date::timestamp, w.historical_days_per_tick_num, w.historical_days_per_tick_den, COALESCE(h.specialization, '')
        FROM households h JOIN worlds w ON w.id=h.world_id
        WHERE h.id=$1::uuid
    `, householdID).Scan(&snap.HouseholdID, &snap.HouseholdName, &snap.WorldID, &snap.WorldName, &snap.CurrentTick, &snap.HistoricalStart, &snap.HistoricalDaysPerTickNum, &snap.HistoricalDaysPerTickDen, &snap.Specialization)
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
        SELECT c.id::text,c.name,c.labor_capacity_milli,c.fatigue,
        COALESCE((SELECT cs.skill_code FROM character_skills cs WHERE cs.character_id=c.id AND cs.level>0 ORDER BY cs.level DESC,cs.skill_code LIMIT 1),'')
        FROM characters c WHERE c.household_id=$1::uuid AND c.status='active' ORDER BY c.created_at,c.id
    `, householdID)
	if err != nil {
		return snap, nil, err
	}
	var chars []simulation.Character
	for rows.Next() {
		var cr CharacterRecord
		if err := rows.Scan(&cr.ID, &cr.Name, &cr.LaborPermille, &cr.Fatigue, &cr.Specialization); err != nil {
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

	var overlap bool
	err = tx.QueryRow(ctx, `
        SELECT EXISTS(
          SELECT 1 FROM assignments
          WHERE character_id=$1::uuid
            AND status IN ('planned','active')
            AND starts_tick <= $3
            AND ends_tick >= $2
        )
    `, characterID, startsTick, endsTick).Scan(&overlap)
	if err != nil {
		return AssignmentRecord{}, err
	}
	if overlap {
		return AssignmentRecord{}, fmt.Errorf("assignment overlaps existing work plan")
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
	if err := tx.Commit(ctx); err != nil {
		return AssignmentRecord{}, err
	}
	return out, nil
}
