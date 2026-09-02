-- +goose Up

ALTER TABLE shipments
    ALTER COLUMN sender_household_id SET NOT NULL,
    ALTER COLUMN receiver_household_id SET NOT NULL,
    ALTER COLUMN departure_tick SET NOT NULL,
    ADD CONSTRAINT shipments_arrival_after_departure
        CHECK (expected_arrival_tick > departure_tick),
    ADD CONSTRAINT shipments_distinct_households
        CHECK (sender_household_id <> receiver_household_id),
    ADD CONSTRAINT shipments_distinct_locations
        CHECK (origin_location_id <> destination_location_id),
    ADD CONSTRAINT shipments_actual_arrival_matches_status
        CHECK (
            (status = 'arrived' AND actual_arrival_tick IS NOT NULL AND actual_arrival_tick >= expected_arrival_tick)
            OR (status <> 'arrived' AND actual_arrival_tick IS NULL)
        );

ALTER TABLE chronicle_entries
    ADD COLUMN related_shipment_id UUID REFERENCES shipments(id);

CREATE UNIQUE INDEX chronicle_shipment_arrival_idx
    ON chronicle_entries(related_shipment_id, entry_type)
    WHERE related_shipment_id IS NOT NULL AND entry_type = 'shipment_arrived';

-- +goose Down

DROP INDEX IF EXISTS chronicle_shipment_arrival_idx;
ALTER TABLE chronicle_entries DROP COLUMN related_shipment_id;
ALTER TABLE shipments
    DROP CONSTRAINT shipments_actual_arrival_matches_status,
    DROP CONSTRAINT shipments_distinct_locations,
    DROP CONSTRAINT shipments_distinct_households,
    DROP CONSTRAINT shipments_arrival_after_departure,
    ALTER COLUMN departure_tick DROP NOT NULL,
    ALTER COLUMN receiver_household_id DROP NOT NULL,
    ALTER COLUMN sender_household_id DROP NOT NULL;
