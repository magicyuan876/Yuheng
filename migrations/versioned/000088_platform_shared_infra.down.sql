-- Rollback: drop the platform-sharing flag from the shared infrastructure
-- tables.
--
-- Rows that were shared simply become ordinary rows of whichever workspace
-- created them. Any other workspace that had selected one keeps a dangling
-- reference (these bindings are bare string columns with no foreign key), so
-- verify nothing is bound cross-tenant before rolling back a deployment that
-- actually used sharing.

DO $$ BEGIN RAISE NOTICE '[Migration 000088 DOWN] Dropping is_builtin from shared infrastructure tables...'; END $$;

DROP INDEX IF EXISTS idx_web_search_providers_builtin;
DROP INDEX IF EXISTS idx_vector_stores_builtin;
DROP INDEX IF EXISTS idx_storage_backends_builtin;

ALTER TABLE web_search_providers DROP COLUMN IF EXISTS is_builtin;
ALTER TABLE vector_stores DROP COLUMN IF EXISTS is_builtin;
ALTER TABLE storage_backends DROP COLUMN IF EXISTS is_builtin;

DO $$ BEGIN RAISE NOTICE '[Migration 000088 DOWN] Done.'; END $$;
