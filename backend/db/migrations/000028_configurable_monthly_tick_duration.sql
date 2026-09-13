-- +goose Up

-- Monthly worlds still advance one game hour per execution tick, but their
-- real-time interval is a deployment/playtest setting. Keep the deterministic
-- game-time ratio while allowing the local playtest to run at 10 seconds/tick.
ALTER TABLE worlds
    DROP CONSTRAINT worlds_monthly_clock_check,
    ADD CONSTRAINT worlds_monthly_clock_check CHECK (
        simulation_model <> 'monthly_seasons_v1'
        OR (
            calendar_anchor_at IS NOT NULL
            AND world_utc_offset_minutes BETWEEN -840 AND 840
            AND tick_duration_seconds > 0
            AND game_days_per_tick_num = 1
            AND game_days_per_tick_den = 24
        )
    );

-- +goose Down

-- Existing monthly rows must be restored to the old schedule before the
-- previous invariant can be recreated.
UPDATE worlds
SET tick_duration_seconds = 3600
WHERE simulation_model = 'monthly_seasons_v1';

ALTER TABLE worlds
    DROP CONSTRAINT worlds_monthly_clock_check,
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
