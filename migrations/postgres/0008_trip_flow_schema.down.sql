ALTER TABLE orders DROP COLUMN IF EXISTS on_way_at;
ALTER TABLE orders DROP COLUMN IF EXISTS arrived_at;
ALTER TABLE orders DROP COLUMN IF EXISTS started_at;
ALTER TABLE orders DROP COLUMN IF EXISTS completed_at;
-- Cannot easily remove values from ENUM in Postgres without dropping type
