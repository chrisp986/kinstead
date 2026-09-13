//go:build postgres

package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

const adminQueryLimit = 101

func adminReadTx(ctx context.Context, pool interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}) (pgx.Tx, error) {
	return pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
}

func (s *Store) ListAdminWorlds(ctx context.Context, cursor port.AdminCursor, limit int) ([]port.AdminWorld, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT w.id::text,w.name,w.current_tick,w.current_game_day,w.simulation_model,
		       w.tick_duration_seconds,w.next_tick_at,
		       (SELECT MAX(p.processed_at) FROM processed_world_ticks p WHERE p.world_id=w.id),
		       w.calendar_anchor_at,w.world_utc_offset_minutes,
		       hb.instance_id::text,hb.started_at,hb.last_seen_at,
		       COALESCE((SELECT jsonb_agg(jsonb_build_object(
				'id',e.id::text,'occurred_at',e.occurred_at,'source',e.source,
				'error_code',e.error_code,'message',e.message,'request_id',e.request_id,
				'worker_instance_id',e.worker_instance_id::text,'world_id',e.world_id::text,
				'household_id',e.household_id::text,'tick',e.tick,'stage',e.stage,
				'occurrence_count',e.occurrence_count,'first_observed_at',e.first_observed_at,
				'last_observed_at',e.last_observed_at) ORDER BY e.last_observed_at DESC,e.id DESC)
				FROM (SELECT id,occurred_at,source,error_code,message,request_id,worker_instance_id,world_id,household_id,tick,stage,occurrence_count,first_observed_at,last_observed_at
				      FROM operational_errors WHERE world_id=w.id ORDER BY last_observed_at DESC,id DESC LIMIT 10) e),'[]'::jsonb)
		FROM worlds w
		LEFT JOIN LATERAL (SELECT instance_id,started_at,last_seen_at FROM worker_heartbeats ORDER BY last_seen_at DESC,instance_id LIMIT 1) hb ON true
		WHERE ($1='' OR (w.name,$1::text) > ($2,$3::text))
		ORDER BY w.name,w.id
		LIMIT $4
	`, cursor.ID, cursor.Name, cursor.ID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminWorld, 0, limit)
	for rows.Next() {
		var item port.AdminWorld
		var heartbeatID *string
		var heartbeatStarted, heartbeatLast *time.Time
		var failures []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.CurrentTick, &item.CurrentGameDay, &item.SimulationModel, &item.TickDurationSeconds, &item.NextTickAt, &item.LastCommittedAt, &item.CalendarAnchorAt, &item.WorldUTCOffsetMinutes, &heartbeatID, &heartbeatStarted, &heartbeatLast, &failures); err != nil {
			return nil, err
		}
		if heartbeatID != nil && heartbeatStarted != nil && heartbeatLast != nil {
			item.LatestHeartbeat = &port.AdminHeartbeat{InstanceID: *heartbeatID, StartedAt: *heartbeatStarted, LastSeenAt: *heartbeatLast}
		}
		item.Heartbeats = []port.AdminHeartbeat{}
		item.RecentFailures = []port.AdminError{}
		if err := json.Unmarshal(failures, &item.RecentFailures); err != nil {
			return nil, err
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, rows.Err()
}

func (s *Store) GetAdminWorld(ctx context.Context, worldID string) (port.AdminWorld, error) {
	tx, err := adminReadTx(ctx, s.Pool)
	if err != nil {
		return port.AdminWorld{}, err
	}
	defer tx.Rollback(ctx)
	var item port.AdminWorld
	err = tx.QueryRow(ctx, `
		SELECT w.id::text,w.name,w.current_tick,w.current_game_day,w.simulation_model,
		       w.tick_duration_seconds,w.next_tick_at,
		       (SELECT MAX(p.processed_at) FROM processed_world_ticks p WHERE p.world_id=w.id),
		       w.calendar_anchor_at,w.world_utc_offset_minutes
		FROM worlds w WHERE w.id=$1::uuid
	`, worldID).Scan(&item.ID, &item.Name, &item.CurrentTick, &item.CurrentGameDay, &item.SimulationModel, &item.TickDurationSeconds, &item.NextTickAt, &item.LastCommittedAt, &item.CalendarAnchorAt, &item.WorldUTCOffsetMinutes)
	if err != nil {
		return port.AdminWorld{}, err
	}
	item.Heartbeats, err = adminHeartbeats(ctx, tx, 20)
	if err != nil {
		return port.AdminWorld{}, err
	}
	if len(item.Heartbeats) > 0 {
		item.LatestHeartbeat = &item.Heartbeats[0]
	}
	item.RecentFailures, err = adminErrors(ctx, tx, port.AdminErrorFilter{WorldID: worldID}, 10)
	if err != nil {
		return port.AdminWorld{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return port.AdminWorld{}, err
	}
	return item, nil
}

func adminHeartbeats(ctx context.Context, tx pgx.Tx, limit int) ([]port.AdminHeartbeat, error) {
	rows, err := tx.Query(ctx, `SELECT instance_id::text,started_at,last_seen_at FROM worker_heartbeats ORDER BY last_seen_at DESC,instance_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminHeartbeat, 0)
	for rows.Next() {
		var item port.AdminHeartbeat
		if err := rows.Scan(&item.InstanceID, &item.StartedAt, &item.LastSeenAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func adminErrors(ctx context.Context, tx pgx.Tx, filter port.AdminErrorFilter, limit int) ([]port.AdminError, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text,occurred_at,source,error_code,message,request_id,
		       worker_instance_id::text,world_id::text,household_id::text,tick,stage,
		       occurrence_count,first_observed_at,last_observed_at
		FROM operational_errors WHERE ($1='' OR world_id=$1::uuid)
		ORDER BY last_observed_at DESC,id DESC LIMIT $2`, filter.WorldID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminError, 0, limit)
	for rows.Next() {
		var item port.AdminError
		if err := rows.Scan(&item.ID, &item.OccurredAt, &item.Source, &item.ErrorCode, &item.Message, &item.RequestID, &item.WorkerInstanceID, &item.WorldID, &item.HouseholdID, &item.Tick, &item.Stage, &item.OccurrenceCount, &item.FirstObservedAt, &item.LastObservedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListAdminHouseholds(ctx context.Context, query string, cursor port.AdminCursor, limit int) ([]port.AdminHousehold, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT h.id::text,h.name,h.owner_player_id::text,h.world_id::text,w.name,
		       w.current_tick,w.current_game_day,w.simulation_model
		FROM households h JOIN worlds w ON w.id=h.world_id
		WHERE ($1='' OR h.name ILIKE '%'||$1||'%' OR h.id::text=$1 OR COALESCE(h.owner_player_id::text,'')=$1)
		  AND ($2='' OR (h.name,$2::text) > ($3,$4::text))
		ORDER BY h.name,h.id LIMIT $5
	`, query, cursor.ID, cursor.Name, cursor.ID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminHousehold, 0, limit)
	for rows.Next() {
		var item port.AdminHousehold
		var gameDay int64
		if err := rows.Scan(&item.ID, &item.Name, &item.OwnerPlayerID, &item.WorldID, &item.WorldName, &item.CurrentCommittedTick, &gameDay, &item.SimulationModel); err != nil {
			return nil, err
		}
		item.ResourcesMilli = map[string]int64{}
		item.PendingOutputMilli = map[string]int64{}
		item.GameMoment = &calendar.Moment{Day: calendar.GameDay(gameDay)}
		item.Characters = []port.AdminCharacter{}
		item.Occupations = []workdomain.Occupation{}
		item.TemporaryDuties = []workdomain.TemporaryDuty{}
		item.IncomingShipments = []port.ShipmentRecord{}
		item.OutgoingShipments = []port.ShipmentRecord{}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, rows.Err()
}

func (s *Store) GetAdminHousehold(ctx context.Context, householdID string) (port.AdminHousehold, error) {
	tx, err := adminReadTx(ctx, s.Pool)
	if err != nil {
		return port.AdminHousehold{}, err
	}
	defer tx.Rollback(ctx)
	var captured time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&captured); err != nil {
		return port.AdminHousehold{}, err
	}
	var owner *string
	if err := tx.QueryRow(ctx, `SELECT owner_player_id::text FROM households WHERE id=$1::uuid`, householdID).Scan(&owner); err != nil {
		return port.AdminHousehold{}, err
	}
	var currentTick int64
	if err := tx.QueryRow(ctx, `SELECT w.current_tick FROM households h JOIN worlds w ON w.id=h.world_id WHERE h.id=$1::uuid`, householdID).Scan(&currentTick); err != nil {
		return port.AdminHousehold{}, err
	}
	snap, _, err := s.LoadHouseholdReadOnly(ctx, tx, householdID, currentTick)
	if err != nil {
		return port.AdminHousehold{}, err
	}
	item, err := adminHouseholdFromSnapshot(snap, owner, captured)
	if err != nil {
		return port.AdminHousehold{}, err
	}
	item.IncomingShipments, item.OutgoingShipments, err = adminHouseholdShipments(ctx, tx, householdID)
	if err != nil {
		return port.AdminHousehold{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT MIN(recorded_at) FROM household_tick_diagnostics WHERE household_id=$1::uuid`, householdID).Scan(&item.HistoryCoverage.DiagnosticsRecordedFrom); err != nil {
		return port.AdminHousehold{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return port.AdminHousehold{}, err
	}
	return item, nil
}

func adminHouseholdFromSnapshot(snap HouseholdSnapshot, owner *string, captured time.Time) (port.AdminHousehold, error) {
	definition, err := calendar.DefinitionForModel(string(snap.SimulationModel))
	if err != nil {
		return port.AdminHousehold{}, err
	}
	moment, momentErr := calendar.MomentAtClock(calendar.ClockState{Day: calendar.GameDay(snap.CurrentGameDay), Remainder: snap.CalendarRemainder, GameDaysPerTickNum: snap.GameDaysPerTickNum, GameDaysPerTickDen: snap.GameDaysPerTickDen})
	if momentErr != nil && snap.SimulationModel.UsesHourlyLabor() {
		return port.AdminHousehold{}, momentErr
	}
	item := port.AdminHousehold{ID: snap.HouseholdID, Name: snap.HouseholdName, OwnerPlayerID: owner, WorldID: snap.WorldID, WorldName: snap.WorldName, SnapshotCapturedAt: captured, CurrentCommittedTick: snap.CurrentTick, SimulationModel: snap.SimulationModel, ResourcesMilli: map[string]int64{"provisions": snap.State.ProvisionsMilli, "wood": snap.State.WoodMilli, "trade_goods": snap.State.TradeGoodsMilli, "silver": snap.State.SilverMilli}, PendingOutputMilli: map[string]int64{}, Characters: []port.AdminCharacter{}, Occupations: []workdomain.Occupation{}, TemporaryDuties: []workdomain.TemporaryDuty{}, HistoryCoverage: port.AdminHistoryCoverage{RetainedDays: 30, Statement: "Tick diagnostics are retained for 30 days; older history is outside retained history."}}
	if snap.SimulationModel.UsesHourlyLabor() && snap.DailyLabor != nil {
		item.GameMoment = &moment
		item.PendingOutputMilli["provisions"] = snap.DailyLabor.PendingProvisionsMilli
		item.PendingOutputMilli["wood"] = snap.DailyLabor.PendingWoodMilli
		contextValue := simulation.DailyLaborWorkContext(moment.Day)
		if snap.SimulationModel == port.ModelMonthlySeasons {
			if snap.CalendarAnchorAt == nil || snap.WorldUTCOffsetMinutes == nil {
				return port.AdminHousehold{}, fmt.Errorf("monthly world %s has no scheduling anchor", snap.WorldID)
			}
			date, err := calendar.SchedulingDate(*snap.CalendarAnchorAt, *snap.WorldUTCOffsetMinutes, moment)
			if err != nil {
				return port.AdminHousehold{}, err
			}
			position, err := calendar.SeasonalPositionForDate(date)
			if err != nil {
				return port.AdminHousehold{}, err
			}
			workday, err := calendar.WorkdayForDate(date, calendar.DefaultDaylightConfig())
			if err != nil {
				return port.AdminHousehold{}, err
			}
			contextValue = simulation.WorkContext{Season: position.Season, Workday: workday}
		}
		item.Season = string(contextValue.Season)
		item.Workday = &contextValue.Workday
		settlement := calendar.Moment{Day: moment.Day, Hour: contextValue.Workday.EndHour}
		if moment.Hour >= contextValue.Workday.EndHour {
			settlement.Day++
		}
		item.NextSettlement = &settlement
	} else {
		item.Season = string(definition.ProductionSeasonAt(calendar.GameDay(snap.CurrentGameDay)))
	}
	for _, c := range snap.Characters {
		age, err := definition.Age(calendar.GameDay(c.BirthGameDay), calendar.GameDay(snap.CurrentGameDay))
		if err != nil {
			return port.AdminHousehold{}, err
		}
		occupation := c.Occupation
		if occupation == nil {
			activity := workdomain.Agriculture
			if c.Specialization == "fishing" {
				activity = workdomain.Fishing
			}
			if c.Specialization == "woodcutting" {
				activity = workdomain.Woodcutting
			}
			occupation = &workdomain.Occupation{CharacterID: c.ID, Activity: activity, Revision: 1}
		}
		item.Occupations = append(item.Occupations, *occupation)
		character := port.AdminCharacter{ID: c.ID, Name: c.Name, Status: c.Status, LaborPermille: c.LaborPermille, Fatigue: c.Fatigue, Age: age, PersistentOccupation: occupation, CurrentActivity: string(occupation.Activity), ActivityReasonCode: "persistent_occupation", ActivityReason: "persistent occupation", TemporaryDutyIDs: []string{}}
		if snap.SimulationModel.UsesHourlyLabor() && snap.DailyLabor != nil {
			for _, dc := range snap.DailyLabor.Characters {
				if dc.ID != c.ID {
					continue
				}
				duties := dc.TemporaryDuties
				resolved, err := workdomain.ResolveEffectiveWork(workdomain.WorkResolutionInput{Moment: moment, Workday: *item.Workday, Status: dc.Status, LaborPermille: dc.LaborPermille, Occupation: dc.Occupation, TemporaryDuties: duties})
				if err != nil {
					return port.AdminHousehold{}, err
				}
				character.CurrentActivity = string(resolved.Activity)
				character.ActivityReason = resolved.Reason
				character.ActivityReasonCode = activityReasonCode(resolved)
				for _, duty := range duties {
					item.TemporaryDuties = append(item.TemporaryDuties, duty)
					character.TemporaryDutyIDs = append(character.TemporaryDutyIDs, duty.ID)
				}
				break
			}
		} else {
			for _, a := range snap.Assignments {
				if a.CharacterID == c.ID && a.StartsTick <= snap.CurrentTick && a.EndsTick >= snap.CurrentTick {
					character.CurrentActivity = a.Activity
					character.ActivityReasonCode = "scheduled_assignment"
					character.ActivityReason = "scheduled assignment"
					break
				}
			}
		}
		if snap.SimulationModel.UsesHourlyLabor() {
			character.ExplanationMoment = &moment
		}
		item.Characters = append(item.Characters, character)
	}
	return item, nil
}

func activityReasonCode(value workdomain.EffectiveWork) string {
	if !value.Eligible {
		return "ineligible"
	}
	if value.BlockedByDuty {
		return "temporary_duty"
	}
	if value.Recovering && value.Reason == "outside working hours" {
		return "outside_work_window"
	}
	if value.Recovering {
		return "unavailable"
	}
	if value.Working {
		return "persistent_occupation"
	}
	return "rest"
}

func adminHouseholdShipments(ctx context.Context, tx pgx.Tx, householdID string) ([]port.ShipmentRecord, []port.ShipmentRecord, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id::text,s.world_id::text,COALESCE(s.sender_household_id::text,''),COALESCE(sh.name,''),COALESCE(s.receiver_household_id::text,''),COALESCE(rh.name,''),s.origin_location_id::text,s.destination_location_id::text,s.resource_code,s.quantity_milli,COALESCE(s.departure_tick,0),s.expected_arrival_tick,s.actual_arrival_tick,COALESCE(s.departure_game_day,0),s.expected_arrival_game_day,s.actual_arrival_game_day,s.transport_cost_milli,s.status
		FROM shipments s LEFT JOIN households sh ON sh.id=s.sender_household_id LEFT JOIN households rh ON rh.id=s.receiver_household_id
		WHERE s.sender_household_id=$1::uuid OR s.receiver_household_id=$1::uuid
		ORDER BY COALESCE(s.actual_arrival_tick,s.expected_arrival_tick) DESC,s.id DESC LIMIT 100`, householdID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	incoming, outgoing := make([]port.ShipmentRecord, 0), make([]port.ShipmentRecord, 0)
	for rows.Next() {
		var value port.ShipmentRecord
		if err := rows.Scan(&value.ID, &value.WorldID, &value.SenderHouseholdID, &value.SenderHouseholdName, &value.ReceiverHouseholdID, &value.ReceiverHouseholdName, &value.OriginLocationID, &value.DestinationLocationID, &value.ResourceType, &value.QuantityMilli, &value.DepartureTick, &value.ExpectedArrivalTick, &value.ActualArrivalTick, &value.DepartureGameDay, &value.ExpectedArrivalGameDay, &value.ActualArrivalGameDay, &value.TransportCostMilli, &value.Status); err != nil {
			return nil, nil, err
		}
		if value.ReceiverHouseholdID == householdID {
			incoming = append(incoming, value)
		} else {
			outgoing = append(outgoing, value)
		}
	}
	return incoming, outgoing, rows.Err()
}

func (s *Store) ListAdminTickDiagnostics(ctx context.Context, householdID string, beforeTick int64, limit int) ([]port.AdminTickDiagnostic, error) {
	rows, err := s.Pool.Query(ctx, `SELECT world_id::text,household_id::text,tick,interval_start_day,interval_start_hour,interval_end_day,interval_end_hour,simulation_model,diagnostic_schema_version,recorded_at,details FROM household_tick_diagnostics WHERE household_id=$1::uuid AND ($2<0 OR tick<$2) ORDER BY tick DESC LIMIT $3`, householdID, beforeTick, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminTickDiagnostic, 0, limit)
	for rows.Next() {
		item, err := scanAdminDiagnostic(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, rows.Err()
}

func (s *Store) GetAdminTickDiagnostic(ctx context.Context, householdID string, tick int64) (port.AdminTickDiagnostic, error) {
	var item port.AdminTickDiagnostic
	var details []byte
	err := s.Pool.QueryRow(ctx, `SELECT world_id::text,household_id::text,tick,interval_start_day,interval_start_hour,interval_end_day,interval_end_hour,simulation_model,diagnostic_schema_version,recorded_at,details FROM household_tick_diagnostics WHERE household_id=$1::uuid AND tick=$2`, householdID, tick).Scan(&item.WorldID, &item.HouseholdID, &item.Tick, &item.IntervalStartDay, &item.IntervalStartHour, &item.IntervalEndDay, &item.IntervalEndHour, &item.SimulationModel, &item.DiagnosticSchemaVersion, &item.RecordedAt, &details)
	if err != nil {
		return item, err
	}
	dec := json.NewDecoder(bytes.NewReader(details))
	dec.UseNumber()
	if err := dec.Decode(&item.Details); err != nil {
		return item, err
	}
	return item, nil
}

type adminRowScanner interface{ Scan(...any) error }

func scanAdminDiagnostic(row adminRowScanner) (port.AdminTickDiagnostic, error) {
	var item port.AdminTickDiagnostic
	var details []byte
	err := row.Scan(&item.WorldID, &item.HouseholdID, &item.Tick, &item.IntervalStartDay, &item.IntervalStartHour, &item.IntervalEndDay, &item.IntervalEndHour, &item.SimulationModel, &item.DiagnosticSchemaVersion, &item.RecordedAt, &details)
	if err != nil {
		return item, err
	}
	dec := json.NewDecoder(bytes.NewReader(details))
	dec.UseNumber()
	if err := dec.Decode(&item.Details); err != nil {
		return item, err
	}
	return item, nil
}

func (s *Store) ListAdminErrors(ctx context.Context, filter port.AdminErrorFilter, cursor port.AdminCursor, limit int) ([]port.AdminError, error) {
	var cursorTime *time.Time
	if cursor.Name != "" {
		v, err := time.Parse(time.RFC3339Nano, cursor.Name)
		if err != nil {
			return nil, err
		}
		cursorTime = &v
	}
	var cursorArg any
	if cursorTime != nil {
		cursorArg = *cursorTime
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text,occurred_at,source,error_code,message,request_id,worker_instance_id::text,world_id::text,household_id::text,tick,stage,occurrence_count,first_observed_at,last_observed_at FROM operational_errors WHERE ($1='' OR world_id=$1::uuid) AND ($2='' OR household_id=$2::uuid) AND ($3::bigint IS NULL OR tick=$3) AND ($4='' OR source=$4) AND ($5='' OR request_id=$5) AND ($6::timestamptz IS NULL OR (last_observed_at<$6::timestamptz OR (last_observed_at=$6::timestamptz AND id<$7::uuid))) ORDER BY last_observed_at DESC,id DESC LIMIT $8`, filter.WorldID, filter.HouseholdID, filter.Tick, filter.Source, filter.RequestID, cursorArg, cursor.ID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminError, 0, limit)
	for rows.Next() {
		var item port.AdminError
		if err := rows.Scan(&item.ID, &item.OccurredAt, &item.Source, &item.ErrorCode, &item.Message, &item.RequestID, &item.WorkerInstanceID, &item.WorldID, &item.HouseholdID, &item.Tick, &item.Stage, &item.OccurrenceCount, &item.FirstObservedAt, &item.LastObservedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, rows.Err()
}

func (s *Store) ListAdminAccounts(ctx context.Context, query string, cursor port.AdminCursor, limit int) ([]port.AdminAccount, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text,external_auth_subject,created_at,updated_at FROM players WHERE ($1='' OR id::text=$1 OR external_auth_subject ILIKE '%'||$1||'%') AND ($2='' OR (external_auth_subject,$2::text) > ($3,$4::text)) ORDER BY external_auth_subject,id LIMIT $5`, query, cursor.ID, cursor.Name, cursor.ID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminAccount, 0, limit)
	for rows.Next() {
		var item port.AdminAccount
		if err := rows.Scan(&item.PlayerID, &item.ExternalAuthSubject, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Households = []port.AdminHouseholdLink{}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, rows.Err()
}

func (s *Store) GetAdminAccount(ctx context.Context, playerID string) (port.AdminAccount, error) {
	var item port.AdminAccount
	err := s.Pool.QueryRow(ctx, `SELECT id::text,external_auth_subject,created_at,updated_at FROM players WHERE id=$1::uuid`, playerID).Scan(&item.PlayerID, &item.ExternalAuthSubject, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text,name FROM households WHERE owner_player_id=$1::uuid ORDER BY name,id LIMIT 100`, playerID)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	item.Households = []port.AdminHouseholdLink{}
	for rows.Next() {
		var h port.AdminHouseholdLink
		if err := rows.Scan(&h.ID, &h.Name); err != nil {
			return item, err
		}
		item.Households = append(item.Households, h)
	}
	return item, rows.Err()
}

func (s *Store) ListAdminSessions(ctx context.Context, playerID string, cursor port.AdminCursor, limit int) ([]port.AdminSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text,player_id::text,created_at,expires_at,revoked_at,last_seen_at,CASE WHEN revoked_at IS NOT NULL THEN 'revoked' WHEN expires_at<=now() THEN 'expired' ELSE 'valid' END FROM player_sessions WHERE player_id=$1::uuid AND ($2='' OR (created_at,$2::text) < (NULLIF($3,'')::timestamptz,$4::text)) ORDER BY created_at DESC,id DESC LIMIT $5`, playerID, cursor.ID, cursor.Name, cursor.ID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.AdminSession, 0, limit)
	for rows.Next() {
		var item port.AdminSession
		if err := rows.Scan(&item.ID, &item.PlayerID, &item.CreatedAt, &item.ExpiresAt, &item.RevokedAt, &item.LastSeenAt, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, rows.Err()
}
