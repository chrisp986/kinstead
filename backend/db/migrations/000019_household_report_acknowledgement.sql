-- +goose Up
ALTER TABLE households ADD COLUMN last_seen_game_day BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE households DROP COLUMN last_seen_game_day;
