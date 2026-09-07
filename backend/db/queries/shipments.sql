-- name: LoadShipmentsDueForArrival :many
SELECT id::text AS id, world_id::text AS world_id,
       sender_household_id::text AS sender_household_id,
       receiver_household_id::text AS receiver_household_id,
       origin_location_id::text AS origin_location_id,
       destination_location_id::text AS destination_location_id,
       resource_code, quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, departure_game_day, expected_arrival_game_day,
       actual_arrival_game_day, transport_cost_milli, status
FROM shipments
WHERE world_id = $1::uuid
  AND status = 'in_transit'
  AND actual_arrival_tick IS NULL
  AND expected_arrival_tick <= $2
ORDER BY expected_arrival_tick, id
FOR UPDATE;

-- name: MarkShipmentArrived :one
UPDATE shipments
SET status = 'arrived', actual_arrival_tick = $2, actual_arrival_game_day = $4
WHERE id = $1::uuid
  AND world_id = $3::uuid
  AND status = 'in_transit'
  AND actual_arrival_tick IS NULL
  AND expected_arrival_tick <= $2
RETURNING id::text;

-- name: ListShipmentsByHousehold :many
SELECT s.id::text AS id, s.world_id::text AS world_id,
       s.sender_household_id::text AS sender_household_id,
       sender.name AS sender_household_name,
       s.receiver_household_id::text AS receiver_household_id,
       receiver.name AS receiver_household_name,
       s.origin_location_id::text AS origin_location_id,
       s.destination_location_id::text AS destination_location_id,
       s.resource_code, s.quantity_milli, s.departure_tick, s.expected_arrival_tick,
       s.actual_arrival_tick, s.departure_game_day, s.expected_arrival_game_day,
       s.actual_arrival_game_day, s.transport_cost_milli, s.status
FROM shipments s
JOIN households sender ON sender.id = s.sender_household_id
JOIN households receiver ON receiver.id = s.receiver_household_id
WHERE s.sender_household_id = $1::uuid OR s.receiver_household_id = $1::uuid
ORDER BY s.departure_tick DESC, s.id;

-- name: ListShipmentsByWorld :many
SELECT id::text AS id, world_id::text AS world_id,
       sender_household_id::text AS sender_household_id,
       receiver_household_id::text AS receiver_household_id,
       origin_location_id::text AS origin_location_id,
       destination_location_id::text AS destination_location_id,
       resource_code, quantity_milli, departure_tick, expected_arrival_tick,
       actual_arrival_tick, departure_game_day, expected_arrival_game_day,
       actual_arrival_game_day, transport_cost_milli, status
FROM shipments
WHERE world_id = $1::uuid
ORDER BY departure_tick DESC, id;
