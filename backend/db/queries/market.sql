-- name: LockMarketOffer :one
SELECT id, world_id, seller_household_id, origin_location_id, resource_code,
       quantity_remaining_milli, price_per_unit_milli, created_tick,
       expires_tick, status
FROM market_offers
WHERE id = $1
FOR UPDATE;

-- name: UpdateMarketOfferAfterPurchase :one
UPDATE market_offers
SET quantity_remaining_milli = $2, status = $3, updated_at = now()
WHERE id = $1 AND status = 'active'
RETURNING id, world_id, seller_household_id, origin_location_id, resource_code,
          quantity_remaining_milli, price_per_unit_milli, created_tick,
          expires_tick, status;

-- name: ListActiveMarketOffers :many
SELECT o.id, o.world_id, o.seller_household_id, o.origin_location_id, o.resource_code,
       o.quantity_remaining_milli, o.price_per_unit_milli, o.created_tick,
       o.expires_tick, o.status
FROM market_offers o
JOIN worlds w ON w.id = o.world_id
WHERE o.world_id = $1
  AND o.status = 'active'
  AND (o.expires_tick IS NULL OR o.expires_tick >= w.current_tick)
ORDER BY o.created_tick, o.id;
