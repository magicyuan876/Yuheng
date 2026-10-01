-- Description: a stored resource is located by its backend and nothing else.
--
-- resources.storage_backend_id becomes required: every object is written
-- through a backend, and reading it back means going to that backend (000134
-- already pointed every row without one at the deployment backend). The
-- provider column goes — it is the backend's provider, recorded a second time
-- per row, and a provider cannot tell two backends apart anyway.
--
-- A location is now the pair (backend, driver path), so its uniqueness is
-- too: the same relative path on two local backends with different path
-- prefixes is two different files. The index on (tenant_id, location_hash)
-- is replaced by one on (storage_backend_id, location_hash). Location hashes
-- written before this release were computed differently; the deployments this
-- release targets are recreated, so no row is rehashed.
DO $$ BEGIN RAISE NOTICE '[Migration 000137] resources: backend required, location unique per backend...'; END $$;

UPDATE resources SET storage_backend_id = 'env' WHERE storage_backend_id IS NULL OR storage_backend_id = '';
ALTER TABLE resources ALTER COLUMN storage_backend_id SET NOT NULL;

DROP INDEX IF EXISTS idx_resources_tenant_location;
CREATE UNIQUE INDEX IF NOT EXISTS idx_resources_backend_location
    ON resources (storage_backend_id, location_hash) WHERE deleted_at IS NULL;

ALTER TABLE resources DROP COLUMN IF EXISTS provider;
