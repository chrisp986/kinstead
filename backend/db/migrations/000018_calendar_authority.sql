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

-- The dropped functions were compatibility shims from earlier migrations.
-- They are intentionally not restored: rolling back this migration must not
-- reintroduce a second, hard-coded calendar implementation.
