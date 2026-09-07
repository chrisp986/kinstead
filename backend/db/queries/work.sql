-- name: ListHouseholdOccupations :many
SELECT c.id::text AS character_id, c.name, c.status, c.labor_capacity_milli,
       COALESCE(o.activity, CASE h.specialization WHEN 'forest' THEN 'woodcutting' ELSE COALESCE(h.specialization, 'agriculture') END) AS activity,
       o.pending_activity, o.effective_game_day, COALESCE(o.revision, 1) AS revision
FROM characters c
JOIN households h ON h.id = c.household_id
LEFT JOIN character_occupations o ON o.character_id = c.id
WHERE c.household_id = sqlc.arg(household_id)::uuid AND c.status <> 'dead'
ORDER BY c.created_at, c.id;

-- name: SaveOccupation :exec
INSERT INTO character_occupations(character_id, activity, pending_activity, effective_game_day, revision)
VALUES (sqlc.arg(character_id)::uuid, sqlc.arg(activity), sqlc.narg(pending_activity), sqlc.narg(effective_game_day), sqlc.arg(revision))
ON CONFLICT (character_id) DO UPDATE SET activity = EXCLUDED.activity,
  pending_activity = EXCLUDED.pending_activity, effective_game_day = EXCLUDED.effective_game_day,
  revision = EXCLUDED.revision;
