-- +goose Up

ALTER TABLE chronicle_entries
    ADD COLUMN related_obligation_id UUID REFERENCES contract_obligations(id);

CREATE UNIQUE INDEX chronicle_obligation_outcome_unique
    ON chronicle_entries(household_id, related_obligation_id, entry_type)
    WHERE related_obligation_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS chronicle_obligation_outcome_unique;
ALTER TABLE chronicle_entries DROP COLUMN IF EXISTS related_obligation_id;
