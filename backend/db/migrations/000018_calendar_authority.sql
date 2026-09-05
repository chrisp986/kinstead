-- +goose Up

-- Normalize legacy civil dates onto the same 364-day calendar used by the
-- current development seed. The mapped day is zero-based within each civil
-- year, preserving the year distance and the birthday's approximate position.
UPDATE characters c
SET birth_game_day =
    (EXTRACT(YEAR FROM c.birth_date)::bigint - w.setting_start_year::bigint) * 364
    + ((EXTRACT(DOY FROM c.birth_date)::bigint - 1) * 364
       / EXTRACT(DOY FROM make_date(EXTRACT(YEAR FROM c.birth_date)::integer, 12, 31))::bigint)
    - ((EXTRACT(DOY FROM w.historical_start_date)::bigint - 1) * 364
       / EXTRACT(DOY FROM make_date(w.setting_start_year, 12, 31))::bigint)
FROM households h
JOIN worlds w ON w.id = h.world_id
WHERE c.household_id = h.id;

-- Game-day values are application/domain results. These compatibility
-- triggers were the last SQL implementations of calendar rules and used a
-- fixed pacing assumption, so remove them after all supported writes are
-- explicit.
DROP TRIGGER IF EXISTS chronicle_entries_fill_game_day ON chronicle_entries;
DROP FUNCTION IF EXISTS game_fill_chronicle_game_day();
DROP TRIGGER IF EXISTS shipments_fill_game_days ON shipments;
DROP FUNCTION IF EXISTS game_fill_shipment_game_days();
DROP TRIGGER IF EXISTS contract_obligations_fill_game_days ON contract_obligations;
DROP FUNCTION IF EXISTS game_fill_obligation_game_days();
DROP TRIGGER IF EXISTS contracts_fill_game_days ON contracts;
DROP FUNCTION IF EXISTS game_fill_contract_game_days();
DROP TRIGGER IF EXISTS relationship_events_fill_game_day ON relationship_events;
DROP FUNCTION IF EXISTS game_fill_relationship_game_day();
DROP TRIGGER IF EXISTS household_decisions_fill_game_days ON household_decisions;
DROP FUNCTION IF EXISTS game_fill_decision_game_days();

-- +goose Down

-- Restore the compatibility boundary that existed at version 17. These are
-- copied from migrations 000014, 000016, and 000017 so a rollback restores
-- the exact pre-000018 schema behavior.
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
