-- name: LoadPoliticalEventsStartingTick :many
SELECT e.id::text AS id, e.world_id::text AS world_id, e.location_id::text AS location_id,
       e.political_actor_id::text AS political_actor_id, e.event_type, e.starts_tick,
       e.ends_tick, ((e.starts_tick * 91) / 12)::bigint AS starts_game_day,
       COALESCE((e.ends_tick * 91) / 12, 0)::bigint AS expires_game_day,
       e.parameters, a.name AS actor_name, a.actor_type
FROM world_events e
JOIN political_actors a ON a.id = e.political_actor_id
WHERE e.world_id = sqlc.arg(world_id)::uuid
  AND e.starts_tick = sqlc.arg(tick)
  AND e.event_type IN ('political_labor_service','political_levy')
  AND e.political_actor_id IS NOT NULL
ORDER BY e.id;

-- name: ListHouseholdsForPoliticalEvent :many
SELECT h.id::text AS household_id, h.world_id::text AS world_id, h.location_id::text AS location_id
FROM households h
JOIN world_events e ON e.location_id = h.location_id AND e.world_id = h.world_id
WHERE e.id = sqlc.arg(world_event_id)::uuid
ORDER BY h.id;

-- name: InsertHouseholdDecision :execrows
INSERT INTO household_decisions(
  household_id, world_id, world_event_id, decision_type,
  available_from_tick, expires_tick, available_from_game_day, expires_game_day,
  status, default_option, parameters
) VALUES (
  sqlc.arg(household_id)::uuid, sqlc.arg(world_id)::uuid, sqlc.arg(world_event_id)::uuid,
  sqlc.arg(decision_type), sqlc.arg(available_from_tick), sqlc.arg(expires_tick),
  sqlc.arg(available_from_game_day), sqlc.arg(expires_game_day),
  'pending', 'refuse', sqlc.arg(parameters)::jsonb
)
ON CONFLICT (household_id, world_event_id) WHERE world_event_id IS NOT NULL DO NOTHING;

-- name: LoadExpiringPoliticalDecisions :many
SELECT d.id::text AS id, d.household_id::text AS household_id, d.world_id::text AS world_id,
       d.world_event_id::text AS world_event_id, d.decision_type, d.available_from_tick,
       d.expires_tick, d.available_from_game_day, d.expires_game_day, d.status,
       d.selected_option, d.default_option, d.standing_delta, d.parameters,
       e.political_actor_id::text AS political_actor_id, e.event_type
FROM household_decisions d
JOIN world_events e ON e.id = d.world_event_id
WHERE d.world_id = sqlc.arg(world_id)::uuid AND d.expires_tick = sqlc.arg(tick)
  AND d.status = 'pending'
ORDER BY d.id
FOR UPDATE OF d;

-- name: LockPoliticalDecision :one
SELECT d.id::text AS id, d.household_id::text AS household_id, d.world_id::text AS world_id,
       d.world_event_id::text AS world_event_id, d.decision_type, d.available_from_tick,
       d.expires_tick, d.available_from_game_day, d.expires_game_day, d.status,
       d.selected_option, d.default_option, d.standing_delta, d.parameters,
       e.political_actor_id::text AS political_actor_id, e.event_type,
       w.current_tick, w.current_game_day
FROM household_decisions d
JOIN world_events e ON e.id = d.world_event_id
JOIN worlds w ON w.id = d.world_id
WHERE d.id = sqlc.arg(decision_id)::uuid AND d.household_id = sqlc.arg(household_id)::uuid
FOR UPDATE OF d, w;

-- name: AutoResolvePoliticalDecision :execrows
UPDATE household_decisions
SET status = 'auto_resolved', selected_option = sqlc.arg(selected_option), resolved_tick = sqlc.arg(resolved_tick),
    standing_delta = sqlc.arg(standing_delta), updated_at = now()
WHERE id = sqlc.arg(decision_id)::uuid AND status = 'pending';

-- name: ResolvePoliticalDecision :execrows
UPDATE household_decisions
SET status = 'resolved', selected_option = sqlc.arg(selected_option), resolved_tick = sqlc.arg(resolved_tick),
    standing_delta = sqlc.arg(standing_delta), related_assignment_id = NULLIF(sqlc.arg(assignment_id)::text, '')::uuid,
    updated_at = now()
WHERE id = sqlc.arg(decision_id)::uuid AND status = 'pending';

-- name: LockPoliticalRelationshipScore :one
SELECT standing FROM political_relationships
WHERE world_id = sqlc.arg(world_id)::uuid AND household_id = sqlc.arg(household_id)::uuid
  AND political_actor_id = sqlc.arg(political_actor_id)::uuid
FOR UPDATE;

-- name: ApplyPoliticalScoreDelta :exec
INSERT INTO political_relationships(world_id, household_id, political_actor_id, standing, updated_at)
VALUES (sqlc.arg(world_id)::uuid, sqlc.arg(household_id)::uuid, sqlc.arg(political_actor_id)::uuid,
        GREATEST(-100, LEAST(100, sqlc.arg(score_delta))), now())
ON CONFLICT (household_id, political_actor_id)
DO UPDATE SET standing = GREATEST(-100, LEAST(100, political_relationships.standing + EXCLUDED.standing)), updated_at = now();

-- name: LockResourceStock :one
SELECT quantity_milli FROM resource_stocks
WHERE household_id = sqlc.arg(household_id)::uuid AND resource_code = sqlc.arg(resource_code)
FOR UPDATE;

-- name: DeductResourceStock :exec
UPDATE resource_stocks SET quantity_milli = quantity_milli - sqlc.arg(amount), updated_at = now()
WHERE household_id = sqlc.arg(household_id)::uuid AND resource_code = sqlc.arg(resource_code)
  AND quantity_milli >= sqlc.arg(amount);

-- name: LoadPoliticalCharacter :one
SELECT c.id::text AS id, c.household_id::text AS household_id, c.status,
       c.labor_capacity_milli
FROM characters c WHERE c.id = sqlc.arg(character_id)::uuid
  AND c.household_id = sqlc.arg(household_id)::uuid FOR UPDATE;

-- name: AssignmentOverlaps :one
SELECT EXISTS(SELECT 1 FROM assignments
 WHERE character_id = sqlc.arg(character_id)::uuid
   AND status IN ('planned','active')
   AND starts_tick <= sqlc.arg(ends_tick) AND ends_tick >= sqlc.arg(starts_tick)) AS overlaps;

-- name: CreateRulerServiceAssignment :one
INSERT INTO assignments(household_id, character_id, activity_type, intensity, starts_tick, ends_tick, status, metadata)
VALUES (sqlc.arg(household_id)::uuid, sqlc.arg(character_id)::uuid, 'ruler_service', 'normal',
        sqlc.arg(starts_tick), sqlc.arg(ends_tick), 'planned', sqlc.arg(metadata)::jsonb)
RETURNING id::text AS id;

-- name: InsertPoliticalChronicle :execrows
INSERT INTO chronicle_entries(
 household_id, occurred_tick, entry_type, related_household_decision_id,
 related_political_actor_id, related_assignment_id, data
) VALUES (sqlc.arg(household_id)::uuid, sqlc.arg(occurred_tick), sqlc.arg(entry_type),
          sqlc.arg(decision_id)::uuid, sqlc.arg(actor_id)::uuid,
          NULLIF(sqlc.arg(assignment_id)::text, '')::uuid, sqlc.arg(data)::jsonb)
ON CONFLICT (household_id, related_household_decision_id, entry_type)
WHERE related_household_decision_id IS NOT NULL DO NOTHING;

-- name: InsertPoliticalReceivedChronicle :exec
INSERT INTO chronicle_entries(household_id, occurred_tick, entry_type,
  related_household_decision_id, related_political_actor_id, data)
SELECT d.household_id, sqlc.arg(occurred_tick), 'political_demand_received', d.id,
       sqlc.arg(actor_id)::uuid, sqlc.arg(data)::jsonb
FROM household_decisions d
WHERE d.household_id = sqlc.arg(household_id)::uuid
  AND d.world_event_id = sqlc.arg(world_event_id)::uuid
ON CONFLICT (household_id, related_household_decision_id, entry_type)
WHERE related_household_decision_id IS NOT NULL DO NOTHING;

-- name: ListPoliticalRelationshipsForHousehold :many
SELECT r.political_actor_id::text AS political_actor_id, a.name AS actor_name, a.actor_type,
       r.standing, r.updated_at
FROM political_relationships r JOIN political_actors a ON a.id = r.political_actor_id
WHERE r.world_id = (SELECT world_id FROM households WHERE id = sqlc.arg(household_id)::uuid)
  AND r.household_id = sqlc.arg(household_id)::uuid ORDER BY a.name;

-- name: ListPoliticalDecisionsForHousehold :many
SELECT d.id::text AS id, d.decision_type, d.available_from_tick, d.expires_tick, d.status,
       d.available_from_game_day, d.expires_game_day, d.selected_option, d.standing_delta, d.parameters,
       e.political_actor_id::text AS political_actor_id, a.name AS actor_name, a.actor_type
FROM household_decisions d JOIN world_events e ON e.id = d.world_event_id
JOIN political_actors a ON a.id = e.political_actor_id
WHERE d.household_id = sqlc.arg(household_id)::uuid
ORDER BY d.expires_tick DESC, d.id DESC;

-- name: ListEligiblePoliticalCharacters :many
SELECT c.id::text AS id, c.name, c.labor_capacity_milli
FROM characters c
JOIN household_decisions d ON d.household_id = c.household_id
WHERE d.id = sqlc.arg(decision_id)::uuid
  AND d.status = 'pending'
  AND d.decision_type = 'political_labor_service'
  AND c.status = 'active' AND c.labor_capacity_milli = 1000
  AND NOT EXISTS (
    SELECT 1 FROM assignments a
    WHERE a.character_id = c.id AND a.status IN ('planned','active')
      AND a.starts_tick <= d.expires_tick + (d.parameters->>'service_ticks')::bigint - 1
      AND a.ends_tick >= d.expires_tick
  )
ORDER BY c.name, c.id;

-- name: HouseholdExists :one
SELECT EXISTS(SELECT 1 FROM households WHERE id = sqlc.arg(household_id)::uuid) AS exists;
