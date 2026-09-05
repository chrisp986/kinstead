-- +goose Up

-- Existing rows are migrated to the calendar schedule. The false default is
-- retained for legacy test/compatibility inserts that still provide only the
-- old tick schedule; new application-created contracts set it explicitly.
ALTER TABLE contracts
    ADD COLUMN game_day_schedule BOOLEAN NOT NULL DEFAULT false;

UPDATE contracts
SET game_day_schedule = true;

-- +goose Down

ALTER TABLE contracts
    DROP COLUMN game_day_schedule;
