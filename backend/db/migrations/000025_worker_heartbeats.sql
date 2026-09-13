-- +goose Up
CREATE TABLE worker_heartbeats (
    instance_id UUID PRIMARY KEY,
    started_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX worker_heartbeats_last_seen_idx ON worker_heartbeats(last_seen_at);

-- +goose Down
DROP TABLE worker_heartbeats;
