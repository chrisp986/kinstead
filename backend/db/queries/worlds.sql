-- name: ClaimDueWorld :one
SELECT id::text AS id, current_tick, tick_duration_seconds, next_tick_at
FROM worlds
WHERE next_tick_at <= now()
ORDER BY next_tick_at
FOR UPDATE SKIP LOCKED
LIMIT 1;

-- name: IsWorldTickProcessed :one
SELECT EXISTS (
    SELECT 1 FROM processed_world_ticks
    WHERE world_id = $1::uuid AND tick = $2
);

-- name: MarkWorldTickProcessed :exec
INSERT INTO processed_world_ticks (world_id, tick)
VALUES ($1::uuid, $2)
ON CONFLICT DO NOTHING;
