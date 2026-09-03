-- +goose Up

-- These constraints extend the already-landed politics runtime schema without
-- rewriting migration 000011. Existing rows are validated on upgrade.
ALTER TABLE political_actors ADD CONSTRAINT political_actors_location_world_fk
    FOREIGN KEY (location_id, world_id) REFERENCES locations(id, world_id);

ALTER TABLE household_decisions ADD CONSTRAINT household_decisions_world_fk
    FOREIGN KEY (world_id) REFERENCES worlds(id);
ALTER TABLE household_decisions ADD CONSTRAINT household_decisions_household_world_fk
    FOREIGN KEY (household_id, world_id) REFERENCES households(id, world_id);

ALTER TABLE world_events ADD CONSTRAINT world_events_id_world_key UNIQUE (id, world_id);
ALTER TABLE world_events ADD CONSTRAINT world_events_location_world_fk
    FOREIGN KEY (location_id, world_id) REFERENCES locations(id, world_id);
ALTER TABLE household_decisions ADD CONSTRAINT household_decisions_event_world_fk
    FOREIGN KEY (world_event_id, world_id) REFERENCES world_events(id, world_id);

-- +goose Down
ALTER TABLE household_decisions DROP CONSTRAINT IF EXISTS household_decisions_event_world_fk;
ALTER TABLE world_events DROP CONSTRAINT IF EXISTS world_events_location_world_fk;
ALTER TABLE world_events DROP CONSTRAINT IF EXISTS world_events_id_world_key;
ALTER TABLE household_decisions DROP CONSTRAINT IF EXISTS household_decisions_household_world_fk;
ALTER TABLE household_decisions DROP CONSTRAINT IF EXISTS household_decisions_world_fk;
ALTER TABLE political_actors DROP CONSTRAINT IF EXISTS political_actors_location_world_fk;
