-- +goose Up
CREATE TABLE household_tick_diagnostics (
    world_id UUID NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    tick BIGINT NOT NULL,
    interval_start_day BIGINT NOT NULL,
    interval_start_hour INTEGER,
    interval_end_day BIGINT NOT NULL,
    interval_end_hour INTEGER,
    simulation_model TEXT NOT NULL,
    diagnostic_schema_version INTEGER NOT NULL CHECK (diagnostic_schema_version > 0),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    details JSONB NOT NULL,
    PRIMARY KEY (world_id, household_id, tick)
);
CREATE INDEX household_tick_diagnostics_history_idx
    ON household_tick_diagnostics(household_id, tick DESC);
CREATE INDEX household_tick_diagnostics_recorded_idx
    ON household_tick_diagnostics(recorded_at);

CREATE TABLE operational_errors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source TEXT NOT NULL CHECK (source IN ('api', 'worker')),
    error_code TEXT NOT NULL,
    message TEXT NOT NULL,
    request_id TEXT,
    worker_instance_id UUID,
    world_id UUID,
    household_id UUID,
    tick BIGINT,
    stage TEXT,
    fingerprint TEXT NOT NULL,
    occurrence_count BIGINT NOT NULL DEFAULT 1 CHECK (occurrence_count > 0),
    first_observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_observed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX operational_errors_observed_idx ON operational_errors(last_observed_at DESC, id DESC);
CREATE INDEX operational_errors_world_idx ON operational_errors(world_id, last_observed_at DESC);
CREATE INDEX operational_errors_request_idx ON operational_errors(request_id, last_observed_at DESC);
CREATE UNIQUE INDEX operational_errors_fingerprint_idx ON operational_errors(fingerprint);

-- +goose Down
DROP TABLE operational_errors;
DROP TABLE household_tick_diagnostics;
