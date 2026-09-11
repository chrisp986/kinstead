-- +goose Up

-- The scheduling calendar belongs to a world. Existing worlds remain
-- unchanged: only monthly_seasons_v1 rows require an anchor and fixed offset.
ALTER TABLE worlds
    DROP CONSTRAINT worlds_simulation_model_check,
    ADD COLUMN calendar_anchor_at TIMESTAMPTZ,
    ADD COLUMN world_utc_offset_minutes INTEGER,
    ADD CONSTRAINT worlds_simulation_model_check
        CHECK (simulation_model IN ('legacy', 'daily_labor_v1', 'monthly_seasons_v1')),
    ADD CONSTRAINT worlds_monthly_clock_check CHECK (
        simulation_model <> 'monthly_seasons_v1'
        OR (
            calendar_anchor_at IS NOT NULL
            AND world_utc_offset_minutes BETWEEN -840 AND 840
            AND tick_duration_seconds = 3600
            AND game_days_per_tick_num = 1
            AND game_days_per_tick_den = 24
        )
    );

-- A day alone was sufficient for daily_labor_v1's fixed 08:00 boundary.
-- Monthly daylight needs the exact whole-hour work start. Existing pending
-- changes retain their original 08:00 meaning.
ALTER TABLE character_occupations
    DROP CONSTRAINT character_occupations_check,
    ADD COLUMN effective_hour INTEGER CHECK (effective_hour BETWEEN 0 AND 23);

UPDATE character_occupations
SET effective_hour = 8
WHERE pending_activity IS NOT NULL;

ALTER TABLE character_occupations
    ADD CONSTRAINT character_occupations_pending_boundary_check CHECK (
        (pending_activity IS NULL AND effective_game_day IS NULL AND effective_hour IS NULL)
        OR
        (pending_activity IS NOT NULL AND effective_game_day IS NOT NULL AND effective_hour IS NOT NULL)
    );

-- +goose Down

ALTER TABLE character_occupations
    DROP CONSTRAINT character_occupations_pending_boundary_check,
    DROP COLUMN effective_hour,
    ADD CONSTRAINT character_occupations_check CHECK (
        (pending_activity IS NULL AND effective_game_day IS NULL)
        OR (pending_activity IS NOT NULL AND effective_game_day IS NOT NULL)
    );

ALTER TABLE worlds
    DROP CONSTRAINT worlds_monthly_clock_check,
    DROP CONSTRAINT worlds_simulation_model_check,
    DROP COLUMN world_utc_offset_minutes,
    DROP COLUMN calendar_anchor_at,
    ADD CONSTRAINT worlds_simulation_model_check
        CHECK (simulation_model IN ('legacy', 'daily_labor_v1'));
