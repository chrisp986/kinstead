-- +goose Up

-- Historical calendar progression is independent from simulation ticks.
-- Default: 48 balancing ticks = 365 historical days.
ALTER TABLE worlds
    ADD COLUMN historical_days_per_tick_num INTEGER NOT NULL DEFAULT 365 CHECK (historical_days_per_tick_num > 0),
    ADD COLUMN historical_days_per_tick_den INTEGER NOT NULL DEFAULT 48 CHECK (historical_days_per_tick_den > 0);

ALTER TABLE characters
    ADD CONSTRAINT characters_household_name_unique UNIQUE (household_id, name);

CREATE INDEX assignments_character_active_window_idx
    ON assignments(character_id, starts_tick, ends_tick)
    WHERE status IN ('planned','active');

-- +goose Down

DROP INDEX IF EXISTS assignments_character_active_window_idx;
ALTER TABLE characters DROP CONSTRAINT characters_household_name_unique;
ALTER TABLE worlds
    DROP COLUMN historical_days_per_tick_den,
    DROP COLUMN historical_days_per_tick_num;
