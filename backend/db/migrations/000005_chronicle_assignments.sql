-- +goose Up

ALTER TABLE chronicle_entries
    ADD COLUMN related_assignment_id UUID REFERENCES assignments(id);

CREATE UNIQUE INDEX chronicle_assignment_event_idx
    ON chronicle_entries(related_assignment_id, entry_type)
    WHERE related_assignment_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS chronicle_assignment_event_idx;
ALTER TABLE chronicle_entries DROP COLUMN related_assignment_id;
