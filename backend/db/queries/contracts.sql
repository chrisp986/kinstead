-- name: CreateContract :one
INSERT INTO contracts(
    world_id, party_a_household_id, party_b_household_id,
    starts_tick, ends_tick, interval_ticks, status
) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
RETURNING id::text AS id, world_id::text AS world_id,
          party_a_household_id::text AS party_a_household_id,
          party_b_household_id::text AS party_b_household_id,
          starts_tick, ends_tick, interval_ticks, status;

-- name: CreateContractTerm :exec
INSERT INTO contract_terms(
    contract_id, debtor_household_id, creditor_household_id,
    resource_code, quantity_milli
) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5);

-- name: GetContract :one
SELECT id::text AS id, world_id::text AS world_id,
       party_a_household_id::text AS party_a_household_id,
       party_b_household_id::text AS party_b_household_id,
       starts_tick, ends_tick, interval_ticks, status
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
       starts_tick, ends_tick, interval_ticks, status
FROM contracts
WHERE party_a_household_id = $1::uuid OR party_b_household_id = $1::uuid
ORDER BY starts_tick DESC, id;

-- name: LockContractForResponse :one
SELECT c.id::text AS id, c.world_id::text AS world_id,
       c.party_a_household_id::text AS party_a_household_id,
       c.party_b_household_id::text AS party_b_household_id,
       c.starts_tick, c.ends_tick, c.interval_ticks, c.status,
       w.current_tick
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
    resource_code, quantity_milli, due_arrival_tick, status
) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
ON CONFLICT (contract_id, debtor_household_id, creditor_household_id, resource_code, due_arrival_tick)
DO NOTHING;

-- name: ListContractObligations :many
SELECT id::text AS id, contract_id::text AS contract_id,
       debtor_household_id::text AS debtor_household_id,
       creditor_household_id::text AS creditor_household_id,
       resource_code, quantity_milli, due_arrival_tick,
       COALESCE(shipment_id::text, ''::text)::text AS shipment_id,
       status, fulfilled_tick
FROM contract_obligations
WHERE contract_id = $1::uuid
ORDER BY due_arrival_tick, debtor_household_id, creditor_household_id, resource_code;

-- name: LoadContractObligationsForTick :many
SELECT c.world_id::text AS world_id,
       o.id::text AS id, o.contract_id::text AS contract_id,
       o.debtor_household_id::text AS debtor_household_id,
       o.creditor_household_id::text AS creditor_household_id,
       o.resource_code, o.quantity_milli, o.due_arrival_tick,
       COALESCE(o.shipment_id::text, ''::text)::text AS shipment_id,
       o.status, o.fulfilled_tick, s.actual_arrival_tick
FROM contract_obligations o
JOIN contracts c ON c.id = o.contract_id
LEFT JOIN shipments s ON s.id = o.shipment_id
WHERE c.world_id = $1::uuid
  AND c.status IN ('active', 'broken')
  AND o.status IN ('pending', 'dispatched', 'late', 'broken')
  AND (o.due_arrival_tick <= $2 OR s.actual_arrival_tick IS NOT NULL)
  AND (o.status <> 'broken' OR (o.fulfilled_tick IS NULL AND s.actual_arrival_tick IS NOT NULL))
  AND (c.status = 'active' OR o.status = 'broken')
ORDER BY o.due_arrival_tick, o.id
FOR UPDATE OF o;

-- name: UpdateContractObligationAssessment :execrows
UPDATE contract_obligations
SET status = sqlc.arg(new_status),
    fulfilled_tick = sqlc.narg(fulfilled_tick)::bigint,
    updated_at = now()
WHERE id = sqlc.arg(id)::uuid
  AND status = sqlc.arg(old_status)
  AND fulfilled_tick IS NOT DISTINCT FROM sqlc.narg(old_fulfilled_tick)::bigint;

-- name: LockContractObligationForDispatch :one
SELECT o.id::text AS id, o.contract_id::text AS contract_id,
       o.debtor_household_id::text AS debtor_household_id,
       o.creditor_household_id::text AS creditor_household_id,
       o.resource_code, o.quantity_milli, o.due_arrival_tick,
       COALESCE(o.shipment_id::text, ''::text)::text AS shipment_id,
       o.status, o.fulfilled_tick,
       c.world_id::text AS world_id, c.status AS contract_status,
       debtor.location_id::text AS origin_location_id,
       creditor.location_id::text AS destination_location_id,
       w.current_tick, gen_random_uuid()::text AS proposed_shipment_id
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
       starts_tick, ends_tick, interval_ticks, status
FROM contracts
WHERE world_id = $1::uuid AND status = 'active'
ORDER BY id
FOR UPDATE;
