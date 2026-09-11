//go:build postgres

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"game/backend/internal/calendar"
	chronicle "game/backend/internal/domain/chronicle"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

type occupationTx struct {
	store *Store
	tx    pgx.Tx
}

func (s *Store) BeginOccupationChange(ctx context.Context) (port.OccupationChangeTransaction, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &occupationTx{store: s, tx: tx}, nil
}

func (t *occupationTx) LoadOccupationChangeContext(ctx context.Context, householdID, characterID string) (port.OccupationChangeContext, error) {
	var out port.OccupationChangeContext
	var model string
	var pending pgtype.Text
	var effective pgtype.Int8
	var effectiveHour pgtype.Int4
	var anchor pgtype.Timestamptz
	var offset pgtype.Int4
	if err := t.tx.QueryRow(ctx, `
		SELECT h.id::text, h.world_id::text, w.simulation_model, w.current_tick,
		       w.current_game_day, w.calendar_remainder, w.game_days_per_tick_num,
		       w.game_days_per_tick_den, w.calendar_anchor_at, w.world_utc_offset_minutes,
		       c.id::text, c.name, c.status,
		       c.labor_capacity_milli, COALESCE(o.activity,
		          CASE h.specialization WHEN 'forest' THEN 'woodcutting' ELSE COALESCE(h.specialization, 'agriculture') END),
		       o.pending_activity, o.effective_game_day, o.effective_hour, COALESCE(o.revision, 1)
		FROM households h
		JOIN worlds w ON w.id = h.world_id
		JOIN characters c ON c.household_id = h.id AND c.id = $2::uuid
		LEFT JOIN character_occupations o ON o.character_id = c.id
		WHERE h.id = $1::uuid
		FOR UPDATE OF w, h, c
	`, householdID, characterID).Scan(
		&out.HouseholdID, &out.WorldID, &model, &out.CurrentTick,
		&out.Clock.Day, &out.Clock.Remainder, &out.Clock.GameDaysPerTickNum,
		&out.Clock.GameDaysPerTickDen, &anchor, &offset, &out.CharacterID, &out.CharacterName,
		&out.CharacterStatus, &out.LaborPermille, &out.Occupation.Activity,
		&pending, &effective, &effectiveHour, &out.Occupation.Revision,
	); err != nil {
		return out, err
	}
	out.Model = port.SimulationModel(model)
	if anchor.Valid {
		out.CalendarAnchorAt = anchor.Time
	}
	if offset.Valid {
		out.WorldUTCOffsetMinutes = int(offset.Int32)
	}
	out.Occupation.CharacterID = out.CharacterID
	if pending.Valid {
		value := workdomain.Activity(pending.String)
		out.Occupation.PendingActivity = &value
	}
	if effective.Valid {
		value := calendar.GameDay(effective.Int64)
		out.Occupation.EffectiveDay = &value
	}
	if effectiveHour.Valid {
		value := int(effectiveHour.Int32)
		out.Occupation.EffectiveHour = &value
	}
	return out, nil
}

func (t *occupationTx) SaveOccupation(ctx context.Context, occupation workdomain.Occupation) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO character_occupations(character_id, activity, pending_activity, effective_game_day, effective_hour, revision)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		ON CONFLICT (character_id) DO UPDATE SET activity=EXCLUDED.activity,
		 pending_activity=EXCLUDED.pending_activity, effective_game_day=EXCLUDED.effective_game_day, effective_hour=EXCLUDED.effective_hour,
		 revision=EXCLUDED.revision
	`, occupation.CharacterID, occupation.Activity, nullableActivity(occupation.PendingActivity), nullableGameDay(occupation.EffectiveDay), nullableInt(occupation.EffectiveHour), occupation.Revision)
	return err
}

func (t *occupationTx) InsertOccupationChronicle(ctx context.Context, entryType string, tick, gameDay int64, characterID string, occupation workdomain.Occupation) error {
	data, _ := json.Marshal(map[string]any{
		"activity": occupation.Activity, "pending_activity": occupation.PendingActivity,
		"effective_game_day": occupation.EffectiveDay, "effective_hour": occupation.EffectiveHour, "revision": occupation.Revision,
	})
	_, err := t.tx.Exec(ctx, `
		INSERT INTO chronicle_entries(household_id, occurred_tick, occurred_game_day, entry_type, subject_character_id, data)
		SELECT household_id, $2, $3, $4, id, $5::jsonb FROM characters WHERE id=$1::uuid
		AND NOT EXISTS (
			SELECT 1 FROM chronicle_entries e WHERE e.subject_character_id=$1::uuid
			AND e.entry_type=$4 AND e.data=$5::jsonb
		)
	`, characterID, tick, gameDay, entryType, data)
	return err
}

func (t *occupationTx) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *occupationTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

func nullableActivity(value *workdomain.Activity) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func nullableGameDay(value *calendar.GameDay) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func (s *Store) GetWorkPlan(ctx context.Context, householdID string) (port.WorkPlan, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return port.WorkPlan{}, err
	}
	defer tx.Rollback(ctx)
	plan, err := s.loadWorkPlan(ctx, tx, householdID)
	if err != nil {
		return port.WorkPlan{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return port.WorkPlan{}, err
	}
	return plan, nil
}

func (s *Store) SaveHouseholdDailyTick(ctx context.Context, tx pgx.Tx, householdID string, result simulation.HourResult) error {
	state := result.State
	for code, quantity := range map[string]int64{
		"provisions": state.ProvisionsMilli, "wood": state.WoodMilli,
		"trade_goods": state.TradeGoodsMilli, "silver": state.SilverMilli,
	} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO resource_stocks(household_id, resource_code, quantity_milli, updated_at)
			VALUES ($1::uuid,$2,$3,now())
			ON CONFLICT (household_id, resource_code) DO UPDATE
			SET quantity_milli=EXCLUDED.quantity_milli, updated_at=now()
		`, householdID, code, quantity); err != nil {
			return err
		}
	}
	productionRemainders, _ := json.Marshal(state.ProductionRemainders)
	fatigueRemainders, _ := json.Marshal(state.FatigueRemainders)
	var settlementDay any
	if state.LastSettlementDay != nil {
		settlementDay = int64(*state.LastSettlementDay)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO household_daily_labor_state(
		 household_id,pending_provisions_milli,pending_wood_milli,production_remainders,
		 consumption_remainder,wood_upkeep_remainder,fatigue_remainders,last_settlement_game_day,policy_reason,updated_at)
		VALUES ($1::uuid,$2,$3,$4::jsonb,$5,$6,$7::jsonb,$8,$9,now())
		ON CONFLICT (household_id) DO UPDATE SET
		 pending_provisions_milli=EXCLUDED.pending_provisions_milli,
		 pending_wood_milli=EXCLUDED.pending_wood_milli,
		 production_remainders=EXCLUDED.production_remainders,
		 consumption_remainder=EXCLUDED.consumption_remainder,
		 wood_upkeep_remainder=EXCLUDED.wood_upkeep_remainder,
		 fatigue_remainders=EXCLUDED.fatigue_remainders,
		 last_settlement_game_day=EXCLUDED.last_settlement_game_day,
		 policy_reason=EXCLUDED.policy_reason, updated_at=now()
	`, householdID, state.PendingProvisionsMilli, state.PendingWoodMilli, productionRemainders,
		state.ConsumptionRemainder, state.WoodUpkeepRemainder, fatigueRemainders, settlementDay, state.PolicyReason); err != nil {
		return err
	}
	for _, c := range state.Characters {
		if _, err := tx.Exec(ctx, `UPDATE characters SET fatigue=$2,updated_at=now() WHERE id=$1::uuid`, c.ID, c.Fatigue); err != nil {
			return err
		}
		if err := upsertOccupation(ctx, tx, c.Occupation); err != nil {
			return err
		}
	}
	for _, fact := range result.Facts {
		data, _ := json.Marshal(fact.Data)
		if _, err := tx.Exec(ctx, `
			INSERT INTO chronicle_entries(household_id,occurred_tick,occurred_game_day,entry_type,data)
			SELECT $1::uuid,$2,$3,$4,$5::jsonb
			WHERE NOT EXISTS (
				SELECT 1 FROM chronicle_entries WHERE household_id=$1::uuid
				AND occurred_tick=$2 AND entry_type=$4 AND data=$5::jsonb
			)`, householdID, state.Tick, int64(state.CurrentGameDay), fact.Type, data); err != nil {
			return err
		}
	}
	if result.Settled && state.LastSettlementDay != nil {
		day := int64(*state.LastSettlementDay)
		var inserted int64
		err := tx.QueryRow(ctx, `
			INSERT INTO household_daily_settlements(household_id,game_day,provisions_milli,wood_milli,summary)
			VALUES ($1::uuid,$2,$3,$4,jsonb_build_object(
			 'produced_provisions_milli',$5::bigint,'produced_wood_milli',$6::bigint,
			 'consumed_provisions_milli',$7::bigint,'consumed_wood_milli',$8::bigint))
			ON CONFLICT (household_id,game_day) DO NOTHING
			RETURNING game_day
		`, householdID, day, result.SettledProvisionsMilli, result.SettledWoodMilli, result.SettledProvisionsMilli, result.SettledWoodMilli, result.SettledConsumedProvisionsMilli, result.SettledConsumedWoodMilli).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]any{"game_day": day, "produced_provisions_milli": result.SettledProvisionsMilli, "produced_wood_milli": result.SettledWoodMilli, "consumed_provisions_milli": result.SettledConsumedProvisionsMilli, "consumed_wood_milli": result.SettledConsumedWoodMilli})
		if _, err := tx.Exec(ctx, `
			INSERT INTO chronicle_entries(household_id,occurred_tick,occurred_game_day,entry_type,data)
			VALUES ($1::uuid,$2,$3,$4,$5::jsonb)
		`, householdID, state.Tick, day, chronicle.DailyProductionSettled, data); err != nil {
			return err
		}
	}
	if result.FoodShortageMilli > 0 {
		_, err := tx.Exec(ctx, `INSERT INTO chronicle_entries(household_id,occurred_tick,occurred_game_day,entry_type,data) VALUES ($1::uuid,$2,$3,'food_shortage',jsonb_build_object('food_shortage_milli',$4::bigint))`, householdID, state.Tick, int64(state.CurrentGameDay), result.FoodShortageMilli)
		return err
	}
	return nil
}

func upsertOccupation(ctx context.Context, tx pgx.Tx, occupation workdomain.Occupation) error {
	if occupation.CharacterID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO character_occupations(character_id,activity,pending_activity,effective_game_day,effective_hour,revision)
		VALUES ($1::uuid,$2,$3,$4,$5,$6)
		ON CONFLICT (character_id) DO UPDATE SET activity=EXCLUDED.activity,
		pending_activity=EXCLUDED.pending_activity,effective_game_day=EXCLUDED.effective_game_day,effective_hour=EXCLUDED.effective_hour,
		revision=EXCLUDED.revision
	`, occupation.CharacterID, occupation.Activity, nullableActivity(occupation.PendingActivity), nullableGameDay(occupation.EffectiveDay), nullableInt(occupation.EffectiveHour), occupation.Revision)
	return err
}

func (s *Store) loadWorkPlan(ctx context.Context, tx pgx.Tx, householdID string) (port.WorkPlan, error) {
	var plan port.WorkPlan
	var model string
	var clock calendar.ClockState
	var anchor pgtype.Timestamptz
	var offset pgtype.Int4
	if err := tx.QueryRow(ctx, `
		SELECT h.id::text, w.current_tick, w.current_game_day, w.calendar_remainder,
		       w.game_days_per_tick_num, w.game_days_per_tick_den, w.simulation_model,
		       w.calendar_anchor_at, w.world_utc_offset_minutes
		FROM households h JOIN worlds w ON w.id=h.world_id WHERE h.id=$1::uuid
	`, householdID).Scan(&plan.HouseholdID, &plan.CurrentTick, &clock.Day, &clock.Remainder, &clock.GameDaysPerTickNum, &clock.GameDaysPerTickDen, &model, &anchor, &offset); err != nil {
		return plan, err
	}
	plan.CurrentGameDay = int64(clock.Day)
	currentMoment, err := calendar.MomentAtClock(clock)
	if err != nil {
		return plan, err
	}
	plan.CurrentMoment = currentMoment
	simulationModel := port.SimulationModel(model)
	if !simulationModel.UsesHourlyLabor() {
		return plan, fmt.Errorf("work plan is unavailable for simulation model %q", model)
	}
	if simulationModel == port.ModelMonthlySeasons {
		if !anchor.Valid || !offset.Valid {
			return plan, fmt.Errorf("monthly world has no scheduling anchor")
		}
		plan.WorldUTCOffsetMinutes = int(offset.Int32)
		date, err := calendar.SchedulingDate(anchor.Time, plan.WorldUTCOffsetMinutes, currentMoment)
		if err != nil {
			return plan, err
		}
		position, err := calendar.SeasonalPositionForDate(date)
		if err != nil {
			return plan, err
		}
		plan.Season, plan.SeasonDay, plan.SeasonLengthDays = position.Season, position.Day, position.LengthDays
		plan.Workday, err = calendar.WorkdayForDate(date, calendar.DefaultDaylightConfig())
		if err != nil {
			return plan, err
		}
		plan.NextWorkingPeriod, err = calendar.NextWorkStartForWorld(anchor.Time, plan.WorldUTCOffsetMinutes, currentMoment, calendar.DefaultDaylightConfig())
		if err != nil {
			return plan, err
		}
	} else {
		contextValue := simulation.DailyLaborWorkContext(currentMoment.Day)
		plan.Season, plan.Workday = contextValue.Season, contextValue.Workday
		plan.NextWorkingPeriod, err = calendar.NextWorkStart(clock)
		if err != nil {
			return plan, err
		}
	}
	plan.NextSettlement = calendar.Moment{Day: currentMoment.Day, Hour: plan.Workday.EndHour}
	if currentMoment.Hour >= plan.Workday.EndHour {
		tomorrow := calendar.Moment{Day: currentMoment.Day + 1, Hour: 0}
		if simulationModel == port.ModelMonthlySeasons {
			date, dateErr := calendar.SchedulingDate(anchor.Time, plan.WorldUTCOffsetMinutes, tomorrow)
			if dateErr != nil {
				return plan, dateErr
			}
			tomorrowWorkday, workErr := calendar.WorkdayForDate(date, calendar.DefaultDaylightConfig())
			if workErr != nil {
				return plan, workErr
			}
			plan.NextSettlement = calendar.Moment{Day: tomorrow.Day, Hour: tomorrowWorkday.EndHour}
		} else {
			plan.NextSettlement = calendar.Moment{Day: tomorrow.Day, Hour: 17}
		}
	}
	rows, err := tx.Query(ctx, `
		SELECT c.id::text, COALESCE(o.activity,
		 CASE h.specialization WHEN 'forest' THEN 'woodcutting' ELSE COALESCE(h.specialization, 'agriculture') END),
		 o.pending_activity, o.effective_game_day, o.effective_hour, COALESCE(o.revision, 1)
		FROM characters c JOIN households h ON h.id=c.household_id
		LEFT JOIN character_occupations o ON o.character_id=c.id
		WHERE c.household_id=$1::uuid AND c.status <> 'dead' ORDER BY c.created_at,c.id
	`, householdID)
	if err != nil {
		return plan, err
	}
	defer rows.Close()
	for rows.Next() {
		var occupation workdomain.Occupation
		var id, activity string
		var pending pgtype.Text
		var effective pgtype.Int8
		var effectiveHour pgtype.Int4
		if err := rows.Scan(&id, &activity, &pending, &effective, &effectiveHour, &occupation.Revision); err != nil {
			return plan, err
		}
		occupation.CharacterID, occupation.Activity = id, workdomain.Activity(activity)
		if pending.Valid {
			value := workdomain.Activity(pending.String)
			occupation.PendingActivity = &value
		}
		if effective.Valid {
			value := calendar.GameDay(effective.Int64)
			occupation.EffectiveDay = &value
		}
		if effectiveHour.Valid {
			value := int(effectiveHour.Int32)
			occupation.EffectiveHour = &value
		}
		plan.Occupations = append(plan.Occupations, occupation)
	}
	if err := rows.Err(); err != nil {
		return plan, err
	}
	dutyRows, err := tx.Query(ctx, `
		SELECT a.id::text, a.character_id::text, a.starts_tick, a.ends_tick
		FROM assignments a WHERE a.household_id=$1::uuid AND a.activity_type='ruler_service'
		  AND a.status IN ('planned','active') AND a.ends_tick >= $2 ORDER BY a.starts_tick,a.id
	`, householdID, plan.CurrentTick)
	if err != nil {
		return plan, err
	}
	defer dutyRows.Close()
	for dutyRows.Next() {
		var id, characterID string
		var startsTick, endsTick int64
		if err := dutyRows.Scan(&id, &characterID, &startsTick, &endsTick); err != nil {
			return plan, err
		}
		start, err := calendar.ShiftMoment(plan.CurrentMoment, startsTick-plan.CurrentTick-1)
		if err != nil {
			return plan, err
		}
		end, err := calendar.ShiftMoment(plan.CurrentMoment, endsTick-plan.CurrentTick)
		if err != nil {
			return plan, err
		}
		plan.TemporaryDuties = append(plan.TemporaryDuties, workdomain.TemporaryDuty{ID: id, CharacterID: characterID, Activity: workdomain.RulerService, Starts: start, Ends: end, Description: "temporary Jarl service"})
	}
	if err := dutyRows.Err(); err != nil {
		return plan, err
	}
	return plan, nil
}
