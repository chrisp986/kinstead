-- name: InsertRelationshipEvent :execrows
INSERT INTO relationship_events(
    world_id, source_household_id, target_household_id,
    event_type, trust_delta, occurred_tick,
    related_contract_id, related_shipment_id, related_obligation_id, data
) VALUES (
    sqlc.arg(world_id)::uuid,
    sqlc.arg(source_household_id)::uuid,
    sqlc.arg(target_household_id)::uuid,
    sqlc.arg(event_type),
    sqlc.arg(trust_delta),
    sqlc.arg(occurred_tick),
    sqlc.arg(contract_id)::uuid,
    NULLIF(sqlc.arg(shipment_id)::text, '')::uuid,
    sqlc.arg(obligation_id)::uuid,
    jsonb_build_object(
        'resource_type', sqlc.arg(resource_type)::text,
        'quantity_milli', sqlc.arg(quantity_milli)::bigint,
        'due_arrival_tick', sqlc.arg(due_arrival_tick)::bigint,
        'actual_fulfillment_tick', sqlc.narg(actual_fulfillment_tick)::bigint
    )
)
ON CONFLICT (source_household_id, target_household_id, event_type, related_obligation_id)
WHERE related_obligation_id IS NOT NULL
DO NOTHING;

-- name: ApplyRelationshipDelta :exec
INSERT INTO relationships(
    world_id, source_household_id, target_household_id,
    trust, first_interaction_tick, updated_at
) VALUES (
    sqlc.arg(world_id)::uuid,
    sqlc.arg(source_household_id)::uuid,
    sqlc.arg(target_household_id)::uuid,
    sqlc.arg(trust_delta),
    sqlc.arg(occurred_tick),
    now()
)
ON CONFLICT (source_household_id, target_household_id)
DO UPDATE SET
    trust = GREATEST(-100, LEAST(100, relationships.trust + EXCLUDED.trust)),
    first_interaction_tick = LEAST(relationships.first_interaction_tick, EXCLUDED.first_interaction_tick),
    updated_at = now();
