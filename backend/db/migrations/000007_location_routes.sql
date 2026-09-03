-- +goose Up

ALTER TABLE locations ADD CONSTRAINT locations_id_world_unique UNIQUE (id, world_id);

CREATE TABLE location_routes (
    world_id UUID NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    origin_location_id UUID NOT NULL,
    destination_location_id UUID NOT NULL,
    distance_class TEXT NOT NULL CHECK (
        distance_class IN ('neighbor','local','near_regional','regional','far_regional','long_distance')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (origin_location_id, destination_location_id),
    FOREIGN KEY (origin_location_id, world_id) REFERENCES locations(id, world_id) ON DELETE CASCADE,
    FOREIGN KEY (destination_location_id, world_id) REFERENCES locations(id, world_id) ON DELETE CASCADE,
    CHECK (origin_location_id <> destination_location_id)
);
CREATE INDEX location_routes_world_idx ON location_routes(world_id);

-- +goose Down

DROP TABLE IF EXISTS location_routes;
ALTER TABLE locations DROP CONSTRAINT locations_id_world_unique;
