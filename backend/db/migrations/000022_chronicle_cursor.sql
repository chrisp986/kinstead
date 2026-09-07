-- +goose Up

-- UUIDs are intentionally opaque and created_at can tie under concurrent
-- transactions.  This monotonic sequence is the household report watermark.
ALTER TABLE chronicle_entries
    ADD COLUMN event_sequence BIGINT GENERATED ALWAYS AS IDENTITY;

ALTER TABLE households
    ADD COLUMN last_seen_chronicle_sequence BIGINT NOT NULL DEFAULT 0;

CREATE INDEX chronicle_entries_household_sequence_idx
    ON chronicle_entries(household_id, event_sequence);

-- +goose Down

DROP INDEX IF EXISTS chronicle_entries_household_sequence_idx;
ALTER TABLE households DROP COLUMN last_seen_chronicle_sequence;
ALTER TABLE chronicle_entries DROP COLUMN event_sequence;
