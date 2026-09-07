-- +goose Up

-- Existing worlds remain frozen on the legacy execution path. New worlds must
-- opt into daily_labor_v1 explicitly; this migration never converts history.
ALTER TABLE worlds
    ADD COLUMN simulation_model TEXT NOT NULL DEFAULT 'legacy'
        CHECK (simulation_model IN ('legacy', 'daily_labor_v1'));

CREATE TABLE character_occupations (
    character_id UUID PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    activity TEXT NOT NULL CHECK (activity IN ('agriculture', 'fishing', 'woodcutting')),
    pending_activity TEXT CHECK (pending_activity IN ('agriculture', 'fishing', 'woodcutting')),
    effective_game_day BIGINT,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    CHECK ((pending_activity IS NULL AND effective_game_day IS NULL)
        OR (pending_activity IS NOT NULL AND effective_game_day IS NOT NULL))
);

CREATE TABLE household_daily_labor_state (
    household_id UUID PRIMARY KEY REFERENCES households(id) ON DELETE CASCADE,
    pending_provisions_milli BIGINT NOT NULL DEFAULT 0 CHECK (pending_provisions_milli >= 0),
    pending_wood_milli BIGINT NOT NULL DEFAULT 0 CHECK (pending_wood_milli >= 0),
    production_remainders JSONB NOT NULL DEFAULT '{}',
    consumption_remainder BIGINT NOT NULL DEFAULT 0 CHECK (consumption_remainder >= 0),
    wood_upkeep_remainder BIGINT NOT NULL DEFAULT 0 CHECK (wood_upkeep_remainder >= 0),
    fatigue_remainders JSONB NOT NULL DEFAULT '{}',
    last_settlement_game_day BIGINT,
    policy_reason TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE household_daily_settlements (
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    game_day BIGINT NOT NULL CHECK (game_day >= 0),
    provisions_milli BIGINT NOT NULL DEFAULT 0 CHECK (provisions_milli >= 0),
    wood_milli BIGINT NOT NULL DEFAULT 0 CHECK (wood_milli >= 0),
    summary JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, game_day)
);

CREATE INDEX character_occupations_activity_idx ON character_occupations(activity);

-- +goose Down

DROP TABLE IF EXISTS household_daily_settlements;
DROP TABLE IF EXISTS household_daily_labor_state;
DROP TABLE IF EXISTS character_occupations;
ALTER TABLE worlds DROP COLUMN simulation_model;
