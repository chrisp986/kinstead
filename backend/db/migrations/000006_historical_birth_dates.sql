-- +goose Up

-- Legacy birth_tick values were historical day offsets (for example -11680
-- for 32 years), despite the column name. Preserve their intended meaning as
-- an explicit historical date before removing the ambiguous representation.
ALTER TABLE characters ADD COLUMN birth_date DATE;
UPDATE characters c
SET birth_date = w.historical_start_date + c.birth_tick::integer
FROM households h, worlds w
WHERE c.household_id = h.id AND h.world_id = w.id;
ALTER TABLE characters ALTER COLUMN birth_date SET NOT NULL;
ALTER TABLE characters DROP COLUMN birth_tick;

-- +goose Down

ALTER TABLE characters ADD COLUMN birth_tick BIGINT;
UPDATE characters c
SET birth_tick = c.birth_date - w.historical_start_date
FROM households h, worlds w
WHERE c.household_id = h.id AND h.world_id = w.id;
ALTER TABLE characters ALTER COLUMN birth_tick SET NOT NULL;
ALTER TABLE characters DROP COLUMN birth_date;
