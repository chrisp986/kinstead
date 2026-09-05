-- +goose Up

ALTER TABLE relationship_events
    ADD COLUMN occurred_game_day BIGINT;

UPDATE relationship_events
SET occurred_game_day = (occurred_tick * 91) / 12;

-- Compatibility inserts from older fixtures still provide an execution tick.
-- New calendar-aware writes provide the immutable game-day snapshot directly.
-- +goose StatementBegin
CREATE FUNCTION game_fill_relationship_game_day() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.occurred_game_day IS NULL THEN
        NEW.occurred_game_day := (NEW.occurred_tick * 91) / 12;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER relationship_events_fill_game_day
BEFORE INSERT OR UPDATE ON relationship_events
FOR EACH ROW EXECUTE FUNCTION game_fill_relationship_game_day();

ALTER TABLE relationship_events
    ALTER COLUMN occurred_game_day SET NOT NULL;

CREATE INDEX relationship_events_game_day_idx
    ON relationship_events(occurred_game_day, id);

-- +goose Down

DROP INDEX IF EXISTS relationship_events_game_day_idx;
DROP TRIGGER IF EXISTS relationship_events_fill_game_day ON relationship_events;
DROP FUNCTION IF EXISTS game_fill_relationship_game_day();
ALTER TABLE relationship_events DROP COLUMN occurred_game_day;
