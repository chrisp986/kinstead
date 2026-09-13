-- +goose Up
CREATE TABLE admin_memberships (
    player_id UUID PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE admin_memberships;
