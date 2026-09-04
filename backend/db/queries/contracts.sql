-- name: CreateContract :one
INSERT INTO contracts(
    world_id, party_a_household_id, party_b_household_id,
    starts_tick, ends_tick, interval_ticks,
    start_game_day, end_game_day, interval_days, game_day_schedule, status
) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id::text AS id, world_id::text AS world_id,
          party_a_household_id::text AS party_a_household_id,
          party_b_household_id::text AS party_b_household_id,
          starts_tick, ends_tick, interval_ticks,
          start_game_day, end_game_day, interval_days, game_day_schedule, status;

-- name: CreateContractTerm :exec
INSERT INTO contract_terms(
    contract_id, debtor_household_id, creditor_household_id,
    resource_code, quantity_milli
) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5);

-- name: GetContract :one
SELECT id::text AS id, world_id::text AS world_id,
       party_a_household_id::text AS party_a_household_id,
       party_b_household_id::text AS party_b_household_id,
       starts_tick, ends_tick, interval_ticks,
       start_game_day, end_game_day, interval_days, game_day_schedule, status
FROM contracts
WHERE id = $1::uuid;

-- name: ListContractTerms :many
SELECT debtor_household_id::text AS debtor_household_id,
       creditor_household_id::text AS creditor_household_id,
       resource_code, quantity_milli
FROM contract_terms
WHERE contract_id = $1::uuid
ORDER BY debtor_household_id, creditor_household_id, resource_code;

-- name: ListContractsForHousehold :many
SELECT id::text AS id, world_id::text AS world_id,
       party_a_household_id::text AS party_a_household_id,
       party_b_household_id::text AS party_b_household_id,
       starts_tick, ends_tick, interval_ticks,
       start_game_day, end_game_day, interval_days, game_day_schedule, status
FROM contracts
WHERE party_a_household_id = $1::uuid OR party_b_household_id = $1::uuid
ORDER BY start_game_day DESC, id;

-- name: LockContractForResponse :one
SELECT c.id::text AS id, c.world_id::text AS world_id,
       c.party_a_household_id::text AS party_a_household_id,
       c.party_b_household_id::text AS party_b_household_id,
       c.starts_tick, c.ends_tick, c.interval_ticks,
       c.start_game_day, c.end_game_day, c.interval_days, c.game_day_schedule, c.status,
       w.current_tick, w.current_game_day
FROM contracts c
JOIN worlds w ON w.id = c.world_id
WHERE c.id = $1::uuid
FOR UPDATE OF c, w;

-- name: UpdateContractStatus :execrows
UPDATE contracts
SET status = $2, updated_at = now()
WHERE id = $1::uuid AND status = $3;

-- name: CreateContractObligation :exec
INSERT INTO contract_obligations(
    contract_id, debtor_household_id, creditor_household_id,
    resource_code, quantity_milli, due_arrival_tick, due_game_day, status
) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8)
ON CONFLICT (contract_id, debtor_household_id, creditor_household_id, resource_code, due_game_day)
DO NOTHING;

-- name: ListContractObligations :many
SELECT o.id::text AS id, o.contract_id::text AS contract_id,
       o.debtor_household_id::text AS debtor_household_id,
       o.creditor_household_id::text AS creditor_household_id,
       o.resource_code, o.quantity_milli, o.due_arrival_tick, o.due_game_day,
       COALESCE(o.shipment_id::text, ''::text)::text AS shipment_id,
       o.status, o.fulfilled_tick, o.fulfilled_game_day, c.game_day_schedule,
       (o.due_game_day - (
         (CASE lr.distance_class
            WHEN 'neighbor' THEN 1 WHEN 'local' THEN 2
            WHEN 'near_regional' THEN 3 WHEN 'regional' THEN 5
            WHEN 'far_regional' THEN 8 ELSE 0 END) * w.game_days_per_tick_num
           + w.game_days_per_tick_den - 1
       ) / w.game_days_per_tick_den)::bigint AS latest_dispatch_game_day,
       s.departure_game_day, s.expected_arrival_game_day
FROM contract_obligations o
JOIN contracts c ON c.id = o.contract_id
JOIN worlds w ON w.id = c.world_id
JOIN households debtor ON debtor.id = o.debtor_household_id
JOIN households creditor ON creditor.id = o.creditor_household_id
LEFT JOIN location_routes lr
  ON lr.world_id = c.world_id
 AND lr.origin_location_id = debtor.location_id
 AND lr.destination_location_id = creditor.location_id
LEFT JOIN shipments s ON s.id = o.shipment_id
WHERE o.contract_id = $1::uuid
ORDER BY o.due_game_day, o.debtor_household_id, o.creditor_household_id, o.resource_code;

-- name: LoadContractObligationsForTick :many
SELECT c.world_id::text AS world_id,
       o.id::text AS id, o.contract_id::text AS contract_id,
       o.debtor_household_id::text AS debtor_household_id,
       o.creditor_household_id::text AS creditor_household_id,
       o.resource_code, o.quantity_milli, o.due_arrival_tick, o.due_game_day,
       COALESCE(o.shipment_id::text, ''::text)::text AS shipment_id,
       o.status, o.fulfilled_tick, o.fulfilled_game_day, c.game_day_schedule,
       s.actual_arrival_tick, s.actual_arrival_game_day
FROM contract_obligations o
JOIN contracts c ON c.id = o.contract_id
JOIN worlds w ON w.id = c.world_id
LEFT JOIN shipments s ON s.id = o.shipment_id
WHERE c.world_id = $1::uuid
  AND c.status IN ('active', 'broken')
  AND o.status IN ('pending', 'dispatched', 'late', 'broken')
  AND ((c.game_day_schedule AND (o.due_game_day <= $2 OR s.actual_arrival_game_day IS NOT NULL))
       OR (NOT c.game_day_schedule AND (o.due_arrival_tick <= w.current_tick OR s.actual_arrival_tick IS NOT NULL)))
  AND ((c.game_day_schedule AND (o.status <> 'broken' OR (o.fulfilled_game_day IS NULL AND s.actual_arrival_game_day IS NOT NULL)))
       OR (NOT c.game_day_schedule AND (o.status <> 'broken' OR (o.fulfilled_tick IS NULL AND s.actual_arrival_tick IS NOT NULL))))
  AND (c.status = 'active' OR o.status = 'broken')
ORDER BY o.due_game_day, o.id
FOR UPDATE OF o;

-- name: UpdateContractObligationAssessment :execrows
UPDATE contract_obligations
SET status = sqlc.arg(new_status),
    fulfilled_tick = sqlc.narg(fulfilled_tick)::bigint,
    fulfilled_game_day = sqlc.narg(fulfilled_game_day)::bigint,
    updated_at = now()
WHERE id = sqlc.arg(id)::uuid
  AND status = sqlc.arg(old_status)
  AND fulfilled_tick IS NOT DISTINCT FROM sqlc.narg(old_fulfilled_tick)::bigint
  AND fulfilled_game_day IS NOT DISTINCT FROM sqlc.narg(old_fulfilled_game_day)::bigint;

-- name: LockContractObligationForDispatch :one
SELECT o.id::text AS id, o.contract_id::text AS contract_id,
       o.debtor_household_id::text AS debtor_household_id,
       o.creditor_household_id::text AS creditor_household_id,
       o.resource_code, o.quantity_milli, o.due_arrival_tick, o.due_game_day,
       COALESCE(o.shipment_id::text, ''::text)::text AS shipment_id,
       o.status, o.fulfilled_tick, o.fulfilled_game_day,
       c.world_id::text AS world_id, c.status AS contract_status,
       debtor.location_id::text AS origin_location_id,
       creditor.location_id::text AS destination_location_id,
       w.current_tick, w.current_game_day, w.game_days_per_tick_num, w.game_days_per_tick_den,
       c.game_day_schedule, gen_random_uuid()::text AS proposed_shipment_id
FROM contract_obligations o
JOIN contracts c ON c.id = o.contract_id
JOIN worlds w ON w.id = c.world_id
JOIN households debtor
  ON debtor.id = o.debtor_household_id AND debtor.world_id = c.world_id
JOIN households creditor
  ON creditor.id = o.creditor_household_id AND creditor.world_id = c.world_id
WHERE o.id = $1::uuid
FOR UPDATE OF o, c, w, debtor, creditor;

-- name: LinkContractObligationShipment :execrows
UPDATE contract_obligations
SET shipment_id = $2::uuid, status = $3, updated_at = now()
WHERE id = $1::uuid AND shipment_id IS NULL AND status = $4;

-- name: ListActiveContractsForRollup :many
SELECT id::text AS id, world_id::text AS world_id,
       party_a_household_id::text AS party_a_household_id,
       party_b_household_id::text AS party_b_household_id,
       starts_tick, ends_tick, interval_ticks,
       start_game_day, end_game_day, interval_days, game_day_schedule, status
FROM contracts
WHERE world_id = $1::uuid AND status = 'active'
ORDER BY id
FOR UPDATE;
