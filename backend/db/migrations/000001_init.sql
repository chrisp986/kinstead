-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE players (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_auth_subject TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE worlds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    historical_start_date DATE NOT NULL,
    current_tick BIGINT NOT NULL DEFAULT 0,
    tick_duration_seconds INTEGER NOT NULL CHECK (tick_duration_seconds > 0),
    next_tick_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX worlds_due_tick_idx ON worlds(next_tick_at);

CREATE TABLE locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    parent_location_id UUID REFERENCES locations(id),
    name TEXT NOT NULL,
    location_type TEXT NOT NULL CHECK (
        location_type IN ('farm','market','settlement','trade_center','ruler_seat','region')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX locations_world_idx ON locations(world_id);

CREATE TABLE households (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    owner_player_id UUID REFERENCES players(id),
    location_id UUID NOT NULL REFERENCES locations(id),
    name TEXT NOT NULL,
    specialization TEXT CHECK (
        specialization IS NULL OR specialization IN ('agriculture','fishing','forest')
    ),
    created_tick BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX households_world_idx ON households(world_id);
CREATE INDEX households_owner_idx ON households(owner_player_id);

CREATE TABLE characters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    birth_tick BIGINT NOT NULL,
    labor_capacity_milli INTEGER NOT NULL CHECK (labor_capacity_milli BETWEEN 0 AND 1000),
    health INTEGER NOT NULL DEFAULT 100 CHECK (health BETWEEN 0 AND 100),
    fatigue INTEGER NOT NULL DEFAULT 0 CHECK (fatigue BETWEEN 0 AND 100),
    status TEXT NOT NULL DEFAULT 'active' CHECK (
        status IN ('active','injured','ill','absent','dead')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX characters_household_idx ON characters(household_id);

CREATE TABLE character_skills (
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_code TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 0 CHECK (level >= 0),
    training_progress INTEGER NOT NULL DEFAULT 0 CHECK (training_progress >= 0),
    PRIMARY KEY (character_id, skill_code)
);

CREATE TABLE resource_types (
    code TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    tradable BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO resource_types(code, category, tradable) VALUES
('provisions','consumable',TRUE),
('wood','material',TRUE),
('trade_goods','goods',TRUE),
('silver','currency',TRUE);

CREATE TABLE resource_stocks (
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    resource_code TEXT NOT NULL REFERENCES resource_types(code),
    quantity_milli BIGINT NOT NULL DEFAULT 0 CHECK (quantity_milli >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (household_id, resource_code)
);

CREATE TABLE assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters(id),
    activity_type TEXT NOT NULL CHECK (
        activity_type IN (
            'agriculture','fishing','woodcutting','crafting','training',
            'market','travel','ruler_service','rest'
        )
    ),
    intensity TEXT NOT NULL CHECK (intensity IN ('light','normal','high')),
    starts_tick BIGINT NOT NULL,
    ends_tick BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'planned' CHECK (
        status IN ('planned','active','completed','cancelled','interrupted')
    ),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_tick >= starts_tick)
);
CREATE INDEX assignments_household_idx ON assignments(household_id);
CREATE INDEX assignments_character_time_idx ON assignments(character_id, starts_tick, ends_tick);
CREATE INDEX assignments_active_idx ON assignments(status, starts_tick, ends_tick);

CREATE TABLE household_buildings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    building_type TEXT NOT NULL CHECK (building_type IN ('storage','workshop','housing')),
    level INTEGER NOT NULL DEFAULT 1 CHECK (level > 0),
    completed_tick BIGINT NOT NULL,
    UNIQUE (household_id, building_type, level)
);

CREATE TABLE market_offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id),
    seller_household_id UUID NOT NULL REFERENCES households(id),
    origin_location_id UUID NOT NULL REFERENCES locations(id),
    resource_code TEXT NOT NULL REFERENCES resource_types(code),
    quantity_remaining_milli BIGINT NOT NULL CHECK (quantity_remaining_milli > 0),
    price_per_unit_milli BIGINT NOT NULL CHECK (price_per_unit_milli > 0),
    created_tick BIGINT NOT NULL,
    expires_tick BIGINT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (
        status IN ('active','filled','cancelled','expired')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX market_offers_search_idx ON market_offers(world_id, resource_code, status);

CREATE TABLE shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id),
    sender_household_id UUID REFERENCES households(id),
    receiver_household_id UUID REFERENCES households(id),
    origin_location_id UUID NOT NULL REFERENCES locations(id),
    destination_location_id UUID NOT NULL REFERENCES locations(id),
    resource_code TEXT NOT NULL REFERENCES resource_types(code),
    quantity_milli BIGINT NOT NULL CHECK (quantity_milli > 0),
    departure_tick BIGINT,
    expected_arrival_tick BIGINT NOT NULL,
    actual_arrival_tick BIGINT,
    transport_cost_milli BIGINT NOT NULL DEFAULT 0 CHECK (transport_cost_milli >= 0),
    status TEXT NOT NULL CHECK (
        status IN ('prepared','in_transit','arrived','cancelled','lost')
    ),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX shipments_due_idx
    ON shipments(world_id, expected_arrival_tick)
    WHERE status = 'in_transit';
CREATE INDEX shipments_receiver_idx ON shipments(receiver_household_id, status);

CREATE TABLE contracts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id),
    party_a_household_id UUID NOT NULL REFERENCES households(id),
    party_b_household_id UUID NOT NULL REFERENCES households(id),
    starts_tick BIGINT NOT NULL,
    ends_tick BIGINT NOT NULL,
    interval_ticks INTEGER NOT NULL CHECK (interval_ticks > 0),
    status TEXT NOT NULL DEFAULT 'proposed' CHECK (
        status IN ('proposed','active','completed','rejected','cancelled','broken')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_tick >= starts_tick),
    CHECK (party_a_household_id <> party_b_household_id)
);

CREATE TABLE contract_obligations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    debtor_household_id UUID NOT NULL REFERENCES households(id),
    creditor_household_id UUID NOT NULL REFERENCES households(id),
    resource_code TEXT NOT NULL REFERENCES resource_types(code),
    quantity_milli BIGINT NOT NULL CHECK (quantity_milli > 0),
    due_arrival_tick BIGINT NOT NULL,
    shipment_id UUID REFERENCES shipments(id),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending','dispatched','fulfilled','late','broken')
    ),
    fulfilled_tick BIGINT
);
CREATE INDEX contract_obligations_due_idx
    ON contract_obligations(due_arrival_tick, status);

CREATE TABLE relationships (
    world_id UUID NOT NULL REFERENCES worlds(id),
    source_household_id UUID NOT NULL REFERENCES households(id),
    target_household_id UUID NOT NULL REFERENCES households(id),
    trust INTEGER NOT NULL DEFAULT 0 CHECK (trust BETWEEN -100 AND 100),
    first_interaction_tick BIGINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (source_household_id, target_household_id),
    CHECK (source_household_id <> target_household_id)
);

CREATE TABLE relationship_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id),
    source_household_id UUID NOT NULL REFERENCES households(id),
    target_household_id UUID NOT NULL REFERENCES households(id),
    event_type TEXT NOT NULL,
    trust_delta INTEGER NOT NULL DEFAULT 0,
    occurred_tick BIGINT NOT NULL,
    related_contract_id UUID REFERENCES contracts(id),
    related_shipment_id UUID REFERENCES shipments(id),
    data JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX relationship_events_pair_idx
    ON relationship_events(source_household_id, target_household_id, occurred_tick);

CREATE TABLE world_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id),
    location_id UUID REFERENCES locations(id),
    event_type TEXT NOT NULL,
    starts_tick BIGINT NOT NULL,
    ends_tick BIGINT,
    parameters JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX world_events_active_idx
    ON world_events(world_id, starts_tick, ends_tick);

CREATE TABLE household_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    world_event_id UUID REFERENCES world_events(id),
    decision_type TEXT NOT NULL,
    available_from_tick BIGINT NOT NULL,
    expires_tick BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending','resolved','expired','auto_resolved')
    ),
    selected_option TEXT,
    default_option TEXT NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}',
    resolved_tick BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_tick >= available_from_tick)
);
CREATE INDEX household_decisions_pending_idx
    ON household_decisions(household_id, status, expires_tick);

CREATE TABLE political_actors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id),
    location_id UUID REFERENCES locations(id),
    name TEXT NOT NULL,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('jarl','ruler'))
);

CREATE TABLE political_relationships (
    household_id UUID NOT NULL REFERENCES households(id),
    political_actor_id UUID NOT NULL REFERENCES political_actors(id),
    standing INTEGER NOT NULL DEFAULT 0 CHECK (standing BETWEEN -100 AND 100),
    PRIMARY KEY (household_id, political_actor_id)
);

CREATE TABLE chronicle_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    occurred_tick BIGINT NOT NULL,
    entry_type TEXT NOT NULL,
    subject_character_id UUID REFERENCES characters(id),
    related_household_id UUID REFERENCES households(id),
    related_contract_id UUID REFERENCES contracts(id),
    data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chronicle_entries_household_idx
    ON chronicle_entries(household_id, occurred_tick DESC);

CREATE TABLE processed_world_ticks (
    world_id UUID NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    tick BIGINT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (world_id, tick)
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID REFERENCES worlds(id),
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX outbox_events_pending_idx
    ON outbox_events(created_at)
    WHERE processed_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS processed_world_ticks;
DROP TABLE IF EXISTS chronicle_entries;
DROP TABLE IF EXISTS political_relationships;
DROP TABLE IF EXISTS political_actors;
DROP TABLE IF EXISTS household_decisions;
DROP TABLE IF EXISTS world_events;
DROP TABLE IF EXISTS relationship_events;
DROP TABLE IF EXISTS relationships;
DROP TABLE IF EXISTS contract_obligations;
DROP TABLE IF EXISTS contracts;
DROP TABLE IF EXISTS shipments;
DROP TABLE IF EXISTS market_offers;
DROP TABLE IF EXISTS household_buildings;
DROP TABLE IF EXISTS assignments;
DROP TABLE IF EXISTS resource_stocks;
DROP TABLE IF EXISTS resource_types;
DROP TABLE IF EXISTS character_skills;
DROP TABLE IF EXISTS characters;
DROP TABLE IF EXISTS households;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS worlds;
DROP TABLE IF EXISTS players;
