-- Down Migration (PostgreSQL doesn't support removing ENUM values easily, so we usually leave them)
-- But we can theoretically recreate the type if needed.
-- For now, we leave it empty or note that it's irreversible without dropping the table.
SELECT 1;
