//go:build postgres

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
	if err := tx.Commit(ctx); err != nil {
		return AssignmentRecord{}, err
	}
	return out, nil
}
