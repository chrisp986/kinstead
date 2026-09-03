-- +goose Up

ALTER TABLE contracts
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE households
    ADD CONSTRAINT households_id_world_unique UNIQUE (id, world_id);

ALTER TABLE contracts
    ADD CONSTRAINT contracts_party_a_world_fk
        FOREIGN KEY (party_a_household_id, world_id) REFERENCES households(id, world_id),
    ADD CONSTRAINT contracts_party_b_world_fk
        FOREIGN KEY (party_b_household_id, world_id) REFERENCES households(id, world_id);

CREATE TABLE contract_terms (
    contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    debtor_household_id UUID NOT NULL REFERENCES households(id),
    creditor_household_id UUID NOT NULL REFERENCES households(id),
    resource_code TEXT NOT NULL REFERENCES resource_types(code),
    quantity_milli BIGINT NOT NULL CHECK (quantity_milli > 0),
    PRIMARY KEY (contract_id, debtor_household_id, creditor_household_id, resource_code),
    CHECK (debtor_household_id <> creditor_household_id)
);

ALTER TABLE contract_obligations
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD CONSTRAINT contract_obligations_schedule_unique
        UNIQUE (contract_id, debtor_household_id, creditor_household_id, resource_code, due_arrival_tick),
    ADD CONSTRAINT contract_obligations_state_consistent CHECK (
        (status = 'pending' AND shipment_id IS NULL AND fulfilled_tick IS NULL)
        OR (status = 'dispatched' AND shipment_id IS NOT NULL AND fulfilled_tick IS NULL)
        OR (status = 'fulfilled' AND shipment_id IS NOT NULL AND fulfilled_tick IS NOT NULL)
        OR (status = 'late' AND (fulfilled_tick IS NULL OR shipment_id IS NOT NULL))
        OR (status = 'broken' AND (fulfilled_tick IS NULL OR shipment_id IS NOT NULL))
    );

CREATE UNIQUE INDEX contract_obligations_shipment_unique
    ON contract_obligations(shipment_id)
    WHERE shipment_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS contract_obligations_shipment_unique;
ALTER TABLE contract_obligations
    DROP CONSTRAINT contract_obligations_state_consistent,
    DROP CONSTRAINT contract_obligations_schedule_unique,
    DROP COLUMN updated_at;
DROP TABLE IF EXISTS contract_terms;
ALTER TABLE contracts
    DROP CONSTRAINT contracts_party_b_world_fk,
    DROP CONSTRAINT contracts_party_a_world_fk,
    DROP COLUMN updated_at;
ALTER TABLE households DROP CONSTRAINT households_id_world_unique;
