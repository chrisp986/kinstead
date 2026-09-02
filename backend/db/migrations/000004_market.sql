-- +goose Up

ALTER TABLE market_offers
    DROP CONSTRAINT market_offers_quantity_remaining_milli_check,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD CONSTRAINT market_offers_quantity_nonnegative
        CHECK (quantity_remaining_milli >= 0),
    ADD CONSTRAINT market_offers_filled_quantity
        CHECK (
            (status = 'filled' AND quantity_remaining_milli = 0)
            OR (status <> 'filled' AND quantity_remaining_milli > 0)
        ),
    ADD CONSTRAINT market_offers_expiry_after_creation
        CHECK (expires_tick IS NULL OR expires_tick > created_tick);

-- +goose Down

ALTER TABLE market_offers
    DROP CONSTRAINT market_offers_expiry_after_creation,
    DROP CONSTRAINT market_offers_filled_quantity,
    DROP CONSTRAINT market_offers_quantity_nonnegative,
    DROP COLUMN updated_at,
    ADD CONSTRAINT market_offers_quantity_remaining_milli_check
        CHECK (quantity_remaining_milli > 0);
