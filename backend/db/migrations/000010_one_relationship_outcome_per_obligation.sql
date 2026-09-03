-- +goose Up

DROP INDEX IF EXISTS relationship_events_obligation_outcome_unique;

CREATE UNIQUE INDEX relationship_events_obligation_outcome_unique
    ON relationship_events(related_obligation_id)
    WHERE related_obligation_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS relationship_events_obligation_outcome_unique;

CREATE UNIQUE INDEX relationship_events_obligation_outcome_unique
    ON relationship_events(source_household_id, target_household_id, event_type, related_obligation_id)
    WHERE related_obligation_id IS NOT NULL;
