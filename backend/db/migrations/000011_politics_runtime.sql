-- +goose Up

-- Runtime Jarl demands. These columns are deliberately separate from the
-- balancing simulator's synthetic political calendar.
ALTER TABLE political_actors ADD CONSTRAINT political_actors_id_world_key UNIQUE (id, world_id);
ALTER TABLE world_events ADD COLUMN political_actor_id UUID;
ALTER TABLE world_events ADD CONSTRAINT world_events_political_actor_fk
    FOREIGN KEY (political_actor_id, world_id) REFERENCES political_actors(id, world_id);

ALTER TABLE household_decisions ADD COLUMN world_id UUID;
UPDATE household_decisions d SET world_id = h.world_id FROM households h WHERE h.id = d.household_id;
ALTER TABLE household_decisions ALTER COLUMN world_id SET NOT NULL;
ALTER TABLE household_decisions ADD COLUMN standing_delta INTEGER;
ALTER TABLE household_decisions ADD COLUMN related_assignment_id UUID REFERENCES assignments(id);
ALTER TABLE household_decisions ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE UNIQUE INDEX household_decisions_household_event_unique
    ON household_decisions(household_id, world_event_id) WHERE world_event_id IS NOT NULL;
ALTER TABLE household_decisions ADD CONSTRAINT household_decisions_state_check CHECK (
    (status = 'pending' AND selected_option IS NULL AND resolved_tick IS NULL AND standing_delta IS NULL)
    OR (status IN ('resolved','auto_resolved') AND selected_option IS NOT NULL AND resolved_tick IS NOT NULL AND standing_delta IS NOT NULL)
    OR status = 'expired'
);

ALTER TABLE political_relationships ADD COLUMN world_id UUID;
UPDATE political_relationships r SET world_id = a.world_id FROM political_actors a WHERE a.id = r.political_actor_id;
ALTER TABLE political_relationships ALTER COLUMN world_id SET NOT NULL;
ALTER TABLE political_relationships ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE political_relationships ADD CONSTRAINT political_relationships_household_world_fk
    FOREIGN KEY (household_id, world_id) REFERENCES households(id, world_id);
ALTER TABLE political_relationships ADD CONSTRAINT political_relationships_actor_world_fk
    FOREIGN KEY (political_actor_id, world_id) REFERENCES political_actors(id, world_id);

ALTER TABLE chronicle_entries ADD COLUMN related_household_decision_id UUID REFERENCES household_decisions(id);
ALTER TABLE chronicle_entries ADD COLUMN related_political_actor_id UUID REFERENCES political_actors(id);
CREATE UNIQUE INDEX chronicle_political_decision_event_unique
    ON chronicle_entries(household_id, related_household_decision_id, entry_type)
    WHERE related_household_decision_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS chronicle_political_decision_event_unique;
ALTER TABLE chronicle_entries DROP COLUMN IF EXISTS related_political_actor_id;
ALTER TABLE chronicle_entries DROP COLUMN IF EXISTS related_household_decision_id;
ALTER TABLE political_relationships DROP CONSTRAINT IF EXISTS political_relationships_actor_world_fk;
ALTER TABLE political_relationships DROP CONSTRAINT IF EXISTS political_relationships_household_world_fk;
ALTER TABLE political_relationships DROP COLUMN IF EXISTS updated_at;
ALTER TABLE political_relationships DROP COLUMN IF EXISTS world_id;
ALTER TABLE household_decisions DROP CONSTRAINT IF EXISTS household_decisions_state_check;
DROP INDEX IF EXISTS household_decisions_household_event_unique;
ALTER TABLE household_decisions DROP COLUMN IF EXISTS updated_at;
ALTER TABLE household_decisions DROP COLUMN IF EXISTS related_assignment_id;
ALTER TABLE household_decisions DROP COLUMN IF EXISTS standing_delta;
ALTER TABLE household_decisions DROP COLUMN IF EXISTS world_id;
ALTER TABLE world_events DROP CONSTRAINT IF EXISTS world_events_political_actor_fk;
ALTER TABLE world_events DROP COLUMN IF EXISTS political_actor_id;
ALTER TABLE political_actors DROP CONSTRAINT IF EXISTS political_actors_id_world_key;
