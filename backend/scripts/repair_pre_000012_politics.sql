-- One-off repair for development databases that applied the temporary,
-- modified version of 000011_politics_runtime.sql. Do not run this as a
-- numbered migration. Apply it only before current 000012 when the marker
-- constraint is absent, for example:
--   psql "$DATABASE_URL" -f backend/scripts/repair_pre_000012_politics.sql
--
-- Databases that already have 000012 are left untouched.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'household_decisions_event_world_fk'
          AND conrelid = 'household_decisions'::regclass
    ) THEN
        ALTER TABLE political_actors
            DROP CONSTRAINT IF EXISTS political_actors_location_world_fk;
        ALTER TABLE household_decisions
            DROP CONSTRAINT IF EXISTS household_decisions_world_fk;
        ALTER TABLE household_decisions
            DROP CONSTRAINT IF EXISTS household_decisions_household_world_fk;
    END IF;
END
$$;
