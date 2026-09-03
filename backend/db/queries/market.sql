-- name: LockMarketOffer :one
SELECT id::text AS id, world_id::text AS world_id,
       seller_household_id::text AS seller_household_id,
       origin_location_id::text AS origin_location_id, resource_code,
       quantity_remaining_milli, price_per_unit_milli, created_tick,
       expires_tick, status
FROM market_offers
WHERE id = $1::uuid
FOR UPDATE;

-- name: UpdateMarketOfferAfterPurchase :one
UPDATE market_offers
SET quantity_remaining_milli = $2, status = $3, updated_at = now()
WHERE id = $1::uuid AND status = 'active' AND quantity_remaining_milli = $4
RETURNING id::text AS id, world_id::text AS world_id,
          seller_household_id::text AS seller_household_id,
          origin_location_id::text AS origin_location_id, resource_code,
          quantity_remaining_milli, price_per_unit_milli, created_tick,
          expires_tick, status;

-- name: ListActiveMarketOffers :many
SELECT o.id::text AS id, o.world_id::text AS world_id,
       o.seller_household_id::text AS seller_household_id,
       o.origin_location_id::text AS origin_location_id, o.resource_code,
       o.quantity_remaining_milli, o.price_per_unit_milli, o.created_tick,
       o.expires_tick, o.status
FROM market_offers o
JOIN worlds w ON w.id = o.world_id
WHERE o.world_id = $1::uuid
  AND o.status = 'active'
  AND (o.expires_tick IS NULL OR o.expires_tick >= w.current_tick)
ORDER BY o.created_tick, o.id;

-- name: GetRouteDistance :one
SELECT distance_class
FROM location_routes
WHERE world_id = $1::uuid
  AND origin_location_id = $2::uuid
  AND destination_location_id = $3::uuid;
