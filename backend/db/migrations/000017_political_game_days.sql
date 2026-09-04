-- +goose Up

-- Political responses keep their tick mechanics for the frozen v0.3 rules, but
-- their player-facing deadline is an immutable calendar snapshot. This keeps a
-- later projection or wall-clock pacing change from moving an already-issued
-- demand in the calendar.
ALTER TABLE household_decisions
    ADD COLUMN available_from_game_day BIGINT,
    ADD COLUMN expires_game_day BIGINT;

UPDATE household_decisions
SET available_from_game_day = (available_from_tick * 91) / 12,
    expires_game_day = (expires_tick * 91) / 12;

-- Older fixtures and direct development inserts may omit the new projection
-- fields. Fill those omissions at the boundary, then enforce the invariant.
-- +goose StatementBegin
CREATE FUNCTION game_fill_decision_game_days() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.available_from_game_day IS NULL THEN
        NEW.available_from_game_day := (NEW.available_from_tick * 91) / 12;
    END IF;
    IF NEW.expires_game_day IS NULL THEN
        NEW.expires_game_day := (NEW.expires_tick * 91) / 12;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER household_decisions_fill_game_days
BEFORE INSERT OR UPDATE ON household_decisions
FOR EACH ROW EXECUTE FUNCTION game_fill_decision_game_days();

ALTER TABLE household_decisions
    ALTER COLUMN available_from_game_day SET NOT NULL,
    ALTER COLUMN expires_game_day SET NOT NULL,
    ADD CONSTRAINT household_decisions_game_day_range_check
        CHECK (expires_game_day >= available_from_game_day);

CREATE INDEX household_decisions_pending_game_day_idx
    ON household_decisions(household_id, status, expires_game_day);

-- +goose Down

DROP INDEX IF EXISTS household_decisions_pending_game_day_idx;
ALTER TABLE household_decisions
    DROP CONSTRAINT IF EXISTS household_decisions_game_day_range_check;
DROP TRIGGER IF EXISTS household_decisions_fill_game_days ON household_decisions;
DROP FUNCTION IF EXISTS game_fill_decision_game_days();
ALTER TABLE household_decisions
    DROP COLUMN IF EXISTS expires_game_day,
    DROP COLUMN IF EXISTS available_from_game_day;
