//go:build postgres

package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"game/backend/internal/port"
)

func (s *Store) LoadHouseholdAccounting(ctx context.Context, tx pgx.Tx, householdID string) (port.HouseholdAccountingState, error) {
	state := port.HouseholdAccountingState{StoredMilli: map[string]int64{}, PendingOutputMilli: map[string]int64{}}
	rows, err := tx.Query(ctx, `SELECT resource_code,quantity_milli FROM resource_stocks WHERE household_id=$1::uuid`, householdID)
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var code string
		var quantity int64
		if err := rows.Scan(&code, &quantity); err != nil {
			rows.Close()
			return state, err
		}
		state.StoredMilli[code] = quantity
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return state, err
	}
	rows.Close()
	var provisions, wood int64
	err = tx.QueryRow(ctx, `SELECT pending_provisions_milli,pending_wood_milli FROM household_daily_labor_state WHERE household_id=$1::uuid`, householdID).Scan(&provisions, &wood)
	if err == nil {
		state.PendingOutputMilli["provisions"], state.PendingOutputMilli["wood"] = provisions, wood
	} else if err != pgx.ErrNoRows {
		return state, err
	}
	return state, nil
}

func (s *Store) PersistHouseholdTickDiagnostic(ctx context.Context, tx pgx.Tx, value port.AdminTickDiagnostic) error {
	details, err := json.Marshal(value.Details)
	if err != nil {
		return err
	}
	if value.DiagnosticSchemaVersion <= 0 {
		return fmt.Errorf("invalid diagnostic schema version")
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO household_tick_diagnostics(world_id,household_id,tick,interval_start_day,interval_start_hour,interval_end_day,interval_end_hour,simulation_model,diagnostic_schema_version,details)
		VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)
	`, value.WorldID, value.HouseholdID, value.Tick, value.IntervalStartDay, value.IntervalStartHour, value.IntervalEndDay, value.IntervalEndHour, value.SimulationModel, value.DiagnosticSchemaVersion, details)
	return err
}
