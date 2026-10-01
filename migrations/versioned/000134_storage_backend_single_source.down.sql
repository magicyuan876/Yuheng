-- Migration 000134 Down: restore the per-workspace schema, with optional
-- bindings. The deployment row goes (its owner-less tenant_id cannot satisfy
-- the restored NOT NULL); bindings that named it are cleared, and
-- legacy_alias returns as false everywhere. The per-workspace copies are not
-- recreated: the previous release's start-up backfill does that.
ALTER TABLE docs_spaces ALTER COLUMN storage_backend_id DROP NOT NULL;
UPDATE docs_spaces SET storage_backend_id = NULL WHERE storage_backend_id = 'env';

ALTER TABLE knowledge_bases ALTER COLUMN storage_backend_id DROP NOT NULL;
UPDATE knowledge_bases SET storage_backend_id = NULL WHERE storage_backend_id = 'env';

ALTER TABLE tenants ALTER COLUMN default_storage_backend_id DROP NOT NULL;
ALTER TABLE tenants ALTER COLUMN default_storage_backend_id DROP DEFAULT;
UPDATE tenants SET default_storage_backend_id = NULL WHERE default_storage_backend_id = 'env';

UPDATE resources SET storage_backend_id = NULL WHERE storage_backend_id = 'env';

DELETE FROM storage_backends WHERE source = 'env';
DROP INDEX IF EXISTS idx_storage_backends_single_env;
ALTER TABLE storage_backends DROP CONSTRAINT IF EXISTS chk_storage_backends_env_owner;
ALTER TABLE storage_backends ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE storage_backends ADD COLUMN IF NOT EXISTS legacy_alias BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_backends_legacy_alias
    ON storage_backends(tenant_id, provider) WHERE deleted_at IS NULL AND legacy_alias = TRUE;
