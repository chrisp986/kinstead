-- name: LoadShipmentsDueForArrival :many
SELECT id::text AS id, world_id::text AS world_id,
       sender_household_id::text AS sender_household_id,
       receiver_household_id::text AS receiver_household_id,
       origin_location_id::text AS origin_location_id,
       destination_location_id::text AS destination_location_id,
       resource_code, quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, transport_cost_milli, status
FROM shipments
WHERE world_id = $1::uuid
  AND status = 'in_transit'
  AND actual_arrival_tick IS NULL
  AND expected_arrival_tick <= $2
ORDER BY expected_arrival_tick, id
FOR UPDATE;

-- name: MarkShipmentArrived :one
UPDATE shipments
SET status = 'arrived', actual_arrival_tick = $2
WHERE id = $1::uuid
  AND world_id = $3::uuid
  AND status = 'in_transit'
  AND actual_arrival_tick IS NULL
  AND expected_arrival_tick <= $2
RETURNING id::text;

-- name: ListShipmentsByHousehold :many
SELECT id::text AS id, world_id::text AS world_id,
       sender_household_id::text AS sender_household_id,
       receiver_household_id::text AS receiver_household_id,
       origin_location_id::text AS origin_location_id,
       destination_location_id::text AS destination_location_id,
       resource_code, quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, transport_cost_milli, status
FROM shipments
WHERE sender_household_id = $1::uuid OR receiver_household_id = $1::uuid
ORDER BY departure_tick DESC, id;

-- name: ListShipmentsByWorld :many
SELECT id::text AS id, world_id::text AS world_id,
       sender_household_id::text AS sender_household_id,
       receiver_household_id::text AS receiver_household_id,
       origin_location_id::text AS origin_location_id,
       destination_location_id::text AS destination_location_id,
       resource_code, quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, transport_cost_milli, status
FROM shipments
WHERE world_id = $1::uuid
ORDER BY departure_tick DESC, id;
