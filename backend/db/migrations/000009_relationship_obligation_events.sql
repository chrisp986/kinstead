-- +goose Up

ALTER TABLE relationships
    ADD CONSTRAINT relationships_source_world_fk
        FOREIGN KEY (source_household_id, world_id) REFERENCES households(id, world_id),
    ADD CONSTRAINT relationships_target_world_fk
        FOREIGN KEY (target_household_id, world_id) REFERENCES households(id, world_id);

ALTER TABLE relationship_events
    ADD COLUMN related_obligation_id UUID REFERENCES contract_obligations(id),
    ADD CONSTRAINT relationship_events_source_world_fk
        FOREIGN KEY (source_household_id, world_id) REFERENCES households(id, world_id),
    ADD CONSTRAINT relationship_events_target_world_fk
        FOREIGN KEY (target_household_id, world_id) REFERENCES households(id, world_id);

CREATE UNIQUE INDEX relationship_events_obligation_outcome_unique
    ON relationship_events(source_household_id, target_household_id, event_type, related_obligation_id)
    WHERE related_obligation_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS relationship_events_obligation_outcome_unique;
ALTER TABLE relationship_events
    DROP CONSTRAINT relationship_events_target_world_fk,
    DROP CONSTRAINT relationship_events_source_world_fk,
    DROP COLUMN related_obligation_id;
ALTER TABLE relationships
    DROP CONSTRAINT relationships_target_world_fk,
    DROP CONSTRAINT relationships_source_world_fk;
