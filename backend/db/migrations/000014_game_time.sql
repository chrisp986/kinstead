-- +goose Up

-- Game time is an absolute, deterministic day count. The legacy civil-date
-- and tick columns remain as compatibility storage for older projections and
-- internal execution mechanics; these columns are the authoritative calendar
-- values from this migration onward.
ALTER TABLE worlds
    ADD COLUMN current_game_day BIGINT NOT NULL DEFAULT 0 CHECK (current_game_day >= 0),
    ADD COLUMN calendar_remainder BIGINT NOT NULL DEFAULT 0 CHECK (calendar_remainder >= 0),
    ADD COLUMN game_days_per_tick_num BIGINT NOT NULL DEFAULT 91 CHECK (game_days_per_tick_num > 0),
    ADD COLUMN game_days_per_tick_den BIGINT NOT NULL DEFAULT 12 CHECK (game_days_per_tick_den > 0),
    ADD COLUMN setting_start_year INTEGER NOT NULL DEFAULT 980;

UPDATE worlds
SET current_game_day = (current_tick * 91) / 12,
    calendar_remainder = (current_tick * 91) % 12,
    game_days_per_tick_num = 91,
    game_days_per_tick_den = 12,
    setting_start_year = EXTRACT(YEAR FROM historical_start_date)::integer;

ALTER TABLE worlds
    ADD CONSTRAINT worlds_calendar_remainder_limit_check
    CHECK (calendar_remainder < game_days_per_tick_den);

ALTER TABLE characters
    ADD COLUMN birth_game_day BIGINT NOT NULL DEFAULT 0;

UPDATE characters c
SET birth_game_day = c.birth_date - w.historical_start_date
FROM households h
JOIN worlds w ON w.id = h.world_id
WHERE c.household_id = h.id;

ALTER TABLE contracts
    ADD COLUMN start_game_day BIGINT,
    ADD COLUMN end_game_day BIGINT,
    ADD COLUMN interval_days INTEGER;

UPDATE contracts
SET start_game_day = (starts_tick * 91) / 12,
    end_game_day = (ends_tick * 91) / 12,
    interval_days = GREATEST(1, (interval_ticks * 91) / 12);

-- +goose StatementBegin
CREATE FUNCTION game_fill_contract_game_days() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.start_game_day IS NULL THEN NEW.start_game_day := (NEW.starts_tick * 91) / 12; END IF;
    IF NEW.end_game_day IS NULL THEN NEW.end_game_day := (NEW.ends_tick * 91) / 12; END IF;
    IF NEW.interval_days IS NULL THEN NEW.interval_days := GREATEST(1, (NEW.interval_ticks * 91) / 12); END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER contracts_fill_game_days
BEFORE INSERT OR UPDATE ON contracts
FOR EACH ROW EXECUTE FUNCTION game_fill_contract_game_days();

ALTER TABLE contracts
    ALTER COLUMN start_game_day SET NOT NULL,
    ALTER COLUMN end_game_day SET NOT NULL,
    ALTER COLUMN interval_days SET NOT NULL,
    ADD CONSTRAINT contracts_game_day_range_check CHECK (end_game_day >= start_game_day),
    ADD CONSTRAINT contracts_interval_days_check CHECK (interval_days > 0);

ALTER TABLE contract_obligations
    ADD COLUMN due_game_day BIGINT,
    ADD COLUMN fulfilled_game_day BIGINT;

UPDATE contract_obligations
SET due_game_day = (due_arrival_tick * 91) / 12,
    fulfilled_game_day = CASE WHEN fulfilled_tick IS NULL THEN NULL ELSE (fulfilled_tick * 91) / 12 END;

-- +goose StatementBegin
CREATE FUNCTION game_fill_obligation_game_days() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.due_game_day IS NULL THEN NEW.due_game_day := (NEW.due_arrival_tick * 91) / 12; END IF;
    IF NEW.fulfilled_game_day IS NULL AND NEW.fulfilled_tick IS NOT NULL THEN
        NEW.fulfilled_game_day := (NEW.fulfilled_tick * 91) / 12;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER contract_obligations_fill_game_days
BEFORE INSERT OR UPDATE ON contract_obligations
FOR EACH ROW EXECUTE FUNCTION game_fill_obligation_game_days();

ALTER TABLE contract_obligations
    ALTER COLUMN due_game_day SET NOT NULL;

CREATE UNIQUE INDEX contract_obligations_game_day_unique
    ON contract_obligations(contract_id, debtor_household_id, creditor_household_id, resource_code, due_game_day);
CREATE INDEX contract_obligations_game_day_idx
    ON contract_obligations(due_game_day, status);

ALTER TABLE shipments
    ADD COLUMN departure_game_day BIGINT,
    ADD COLUMN expected_arrival_game_day BIGINT,
    ADD COLUMN actual_arrival_game_day BIGINT;

UPDATE shipments s
SET departure_game_day = CASE WHEN s.departure_tick IS NULL THEN NULL ELSE (s.departure_tick * 91) / 12 END,
    expected_arrival_game_day = (s.expected_arrival_tick * 91) / 12,
    actual_arrival_game_day = CASE WHEN s.actual_arrival_tick IS NULL THEN NULL ELSE (s.actual_arrival_tick * 91) / 12 END;

-- +goose StatementBegin
CREATE FUNCTION game_fill_shipment_game_days() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.departure_game_day IS NULL AND NEW.departure_tick IS NOT NULL THEN
        NEW.departure_game_day := (NEW.departure_tick * 91) / 12;
    END IF;
    IF NEW.expected_arrival_game_day IS NULL THEN
        NEW.expected_arrival_game_day := (NEW.expected_arrival_tick * 91) / 12;
    END IF;
    IF NEW.actual_arrival_game_day IS NULL AND NEW.actual_arrival_tick IS NOT NULL THEN
        NEW.actual_arrival_game_day := (NEW.actual_arrival_tick * 91) / 12;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER shipments_fill_game_days
BEFORE INSERT OR UPDATE ON shipments
FOR EACH ROW EXECUTE FUNCTION game_fill_shipment_game_days();

ALTER TABLE shipments
    ALTER COLUMN expected_arrival_game_day SET NOT NULL;

ALTER TABLE chronicle_entries
    ADD COLUMN occurred_game_day BIGINT;

UPDATE chronicle_entries
SET occurred_game_day = (occurred_tick * 91) / 12;

-- +goose StatementBegin
CREATE FUNCTION game_fill_chronicle_game_day() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.occurred_game_day IS NULL THEN NEW.occurred_game_day := (NEW.occurred_tick * 91) / 12; END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER chronicle_entries_fill_game_day
BEFORE INSERT OR UPDATE ON chronicle_entries
FOR EACH ROW EXECUTE FUNCTION game_fill_chronicle_game_day();

ALTER TABLE chronicle_entries
    ALTER COLUMN occurred_game_day SET NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS contract_obligations_game_day_idx;
DROP INDEX IF EXISTS contract_obligations_game_day_unique;
DROP TRIGGER IF EXISTS chronicle_entries_fill_game_day ON chronicle_entries;
DROP FUNCTION IF EXISTS game_fill_chronicle_game_day();
DROP TRIGGER IF EXISTS shipments_fill_game_days ON shipments;
DROP FUNCTION IF EXISTS game_fill_shipment_game_days();
DROP TRIGGER IF EXISTS contract_obligations_fill_game_days ON contract_obligations;
DROP FUNCTION IF EXISTS game_fill_obligation_game_days();
DROP TRIGGER IF EXISTS contracts_fill_game_days ON contracts;
DROP FUNCTION IF EXISTS game_fill_contract_game_days();
ALTER TABLE chronicle_entries DROP COLUMN occurred_game_day;
ALTER TABLE shipments
    DROP COLUMN actual_arrival_game_day,
    DROP COLUMN expected_arrival_game_day,
    DROP COLUMN departure_game_day;
ALTER TABLE contract_obligations
    DROP COLUMN fulfilled_game_day,
    DROP COLUMN due_game_day;
ALTER TABLE contracts
    DROP CONSTRAINT contracts_interval_days_check,
    DROP CONSTRAINT contracts_game_day_range_check,
    DROP COLUMN interval_days,
    DROP COLUMN end_game_day,
    DROP COLUMN start_game_day;
ALTER TABLE characters DROP COLUMN birth_game_day;
ALTER TABLE worlds
    DROP CONSTRAINT worlds_calendar_remainder_limit_check,
    DROP COLUMN setting_start_year,
    DROP COLUMN game_days_per_tick_den,
    DROP COLUMN game_days_per_tick_num,
    DROP COLUMN calendar_remainder,
    DROP COLUMN current_game_day;
