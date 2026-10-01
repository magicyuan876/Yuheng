-- Description: storage_backends becomes the only place storage is configured.
--
-- Until now the deployment's environment storage (STORAGE_TYPE, S3_*) was
-- copied into every workspace as its own storage_backends row, re-synced at
-- start-up and marked legacy_alias, next to aliases projected from each
-- workspace's storage_engine_config. Those copies go. One row with id 'env'
-- and no owning workspace stands for the deployment's storage; the server
-- rewrites it from the environment on every start and never stores its
-- credentials. It is shared (is_builtin) so every workspace can use it.
--
-- Bindings become required: every workspace has a default backend, every
-- knowledge base and docs space names the backend its new files go to.
-- Whatever pointed at a removed copy, or at nothing, points at 'env' — the
-- copies were snapshots of exactly that storage. Stored resources written
-- through a copy, or through the environment storage directly (no backend
-- id), are repointed the same way so they stay readable.
DO $$ BEGIN RAISE NOTICE '[Migration 000134] storage_backends: one deployment row replaces per-workspace copies...'; END $$;

UPDATE resources SET storage_backend_id = 'env'
 WHERE storage_backend_id IS NULL OR storage_backend_id = ''
    OR storage_backend_id IN (SELECT id FROM storage_backends WHERE legacy_alias OR source = 'env');

DELETE FROM storage_backends WHERE legacy_alias OR source = 'env';

DROP INDEX IF EXISTS idx_storage_backends_legacy_alias;
ALTER TABLE storage_backends DROP COLUMN IF EXISTS legacy_alias;

ALTER TABLE storage_backends ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE storage_backends ADD CONSTRAINT chk_storage_backends_env_owner
    CHECK ((source = 'env') = (tenant_id IS NULL));
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_backends_single_env
    ON storage_backends (source) WHERE source = 'env';

-- A placeholder: the server overwrites provider, name and config from the
-- environment before it serves anything.
INSERT INTO storage_backends (id, tenant_id, name, provider, config, source, status, is_builtin, created_at, updated_at)
VALUES ('env', NULL, 'Deployment storage', 'local', '{}', 'env', 'active', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

UPDATE tenants SET default_storage_backend_id = 'env'
 WHERE default_storage_backend_id IS NULL
    OR default_storage_backend_id NOT IN (SELECT id FROM storage_backends WHERE deleted_at IS NULL);
ALTER TABLE tenants ALTER COLUMN default_storage_backend_id SET DEFAULT 'env';
ALTER TABLE tenants ALTER COLUMN default_storage_backend_id SET NOT NULL;

UPDATE knowledge_bases SET storage_backend_id = 'env'
 WHERE storage_backend_id IS NULL
    OR storage_backend_id NOT IN (SELECT id FROM storage_backends WHERE deleted_at IS NULL);
ALTER TABLE knowledge_bases ALTER COLUMN storage_backend_id SET NOT NULL;

UPDATE docs_spaces SET storage_backend_id = 'env'
 WHERE storage_backend_id IS NULL
    OR storage_backend_id NOT IN (SELECT id FROM storage_backends WHERE deleted_at IS NULL);
ALTER TABLE docs_spaces ALTER COLUMN storage_backend_id SET NOT NULL;
