-- +goose Up
ALTER TABLE player_sessions
    ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN last_seen_at TIMESTAMPTZ;
CREATE UNIQUE INDEX player_sessions_id_idx ON player_sessions(id);

-- +goose Down
DROP INDEX player_sessions_id_idx;
ALTER TABLE player_sessions
    DROP COLUMN last_seen_at,
    DROP COLUMN id;
