-- Migration 000137 Down: restore the provider column, the per-tenant location
-- index and the optional backend id. The provider is filled from each row's
-- backend, where that backend still exists, and from the locator's scheme
-- otherwise, so the restored NOT NULL holds.
ALTER TABLE resources ADD COLUMN IF NOT EXISTS provider VARCHAR(32);
UPDATE resources r SET provider = b.provider
  FROM storage_backends b WHERE b.id = r.storage_backend_id;
UPDATE resources SET provider = split_part(physical_path, '://', 1) WHERE provider IS NULL;
ALTER TABLE resources ALTER COLUMN provider SET NOT NULL;

DROP INDEX IF EXISTS idx_resources_backend_location;
CREATE UNIQUE INDEX IF NOT EXISTS idx_resources_tenant_location
    ON resources(tenant_id, location_hash) WHERE deleted_at IS NULL;

ALTER TABLE resources ALTER COLUMN storage_backend_id DROP NOT NULL;
