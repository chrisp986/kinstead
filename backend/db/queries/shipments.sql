-- name: LoadShipmentsDueForArrival :many
SELECT id, world_id, sender_household_id, receiver_household_id,
       origin_location_id, destination_location_id, resource_code,
       quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, transport_cost_milli, status
FROM shipments
WHERE world_id = $1
  AND status = 'in_transit'
  AND actual_arrival_tick IS NULL
  AND expected_arrival_tick <= $2
ORDER BY expected_arrival_tick, id
FOR UPDATE;

-- name: MarkShipmentArrived :one
UPDATE shipments
SET status = 'arrived', actual_arrival_tick = $2
WHERE id = $1
  AND status = 'in_transit'
  AND actual_arrival_tick IS NULL
  AND expected_arrival_tick <= $2
RETURNING id;

-- name: ListShipmentsByHousehold :many
SELECT id, world_id, sender_household_id, receiver_household_id,
       origin_location_id, destination_location_id, resource_code,
       quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, transport_cost_milli, status
FROM shipments
WHERE sender_household_id = $1 OR receiver_household_id = $1
ORDER BY departure_tick DESC, id;

-- name: ListShipmentsByWorld :many
SELECT id, world_id, sender_household_id, receiver_household_id,
       origin_location_id, destination_location_id, resource_code,
       quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, transport_cost_milli, status
FROM shipments
WHERE world_id = $1
ORDER BY departure_tick DESC, id;
