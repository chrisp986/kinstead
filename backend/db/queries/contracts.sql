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
