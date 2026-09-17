-- Description: platform-shared infrastructure rows.
--
-- Adds is_builtin to web_search_providers, vector_stores and storage_backends,
-- mirroring the flag models and mcp_services already carry. A row with
-- is_builtin = true is visible to EVERY workspace; reads become
-- "(tenant_id = ? OR is_builtin = true)" while writes stay pinned to the
-- owning tenant and, at the service layer, to system administrators.
--
-- Why this is needed at all: every self-registered user is Owner of their own
-- personal workspace, so the Owner/Admin/Contributor/Viewer ladder cannot
-- express "only the platform operator configures the shared model endpoint /
-- vector store / object storage". Sharing is the other half of that story —
-- once workspace admins lose the write path (governance.centralized_infra),
-- they need platform-provided instances to select at point of use.
--
-- NOT included, deliberately: tenant_sandbox_configs. Skills are stored per
-- (tenant_id, sandbox_config_id) but the image they are baked into is
-- recorded on the CONFIG row (skill_image.snapshot_id). Sharing one config
-- across workspaces would let one workspace's skill install rewrite the image
-- another workspace's sessions boot from, and leave that workspace's skill
-- list disagreeing with what its image actually contains. Sandbox configs stay
-- per-workspace; centralised mode still restricts who may write them.
--
-- Defaults to FALSE everywhere, so an existing deployment sees no change until
-- a system administrator shares something explicitly.

DO $$ BEGIN RAISE NOTICE '[Migration 000088] Adding is_builtin to shared infrastructure tables...'; END $$;

ALTER TABLE web_search_providers
    ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE vector_stores
    ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE storage_backends
    ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN web_search_providers.is_builtin IS
    'Platform-shared: visible to every workspace; only system administrators may write it';
COMMENT ON COLUMN vector_stores.is_builtin IS
    'Platform-shared: visible to every workspace; only system administrators may write it';
COMMENT ON COLUMN storage_backends.is_builtin IS
    'Platform-shared: visible to every workspace; only system administrators may write it';

-- Partial indexes: the shared rows are expected to be a handful among many
-- per-tenant ones, and every list query ORs this predicate against tenant_id.
-- Indexing only the TRUE rows keeps these tiny while still letting the planner
-- satisfy the OR branch without a sequential scan.
CREATE INDEX IF NOT EXISTS idx_web_search_providers_builtin
    ON web_search_providers (is_builtin) WHERE is_builtin = TRUE;
CREATE INDEX IF NOT EXISTS idx_vector_stores_builtin
    ON vector_stores (is_builtin) WHERE is_builtin = TRUE;
CREATE INDEX IF NOT EXISTS idx_storage_backends_builtin
    ON storage_backends (is_builtin) WHERE is_builtin = TRUE;

DO $$ BEGIN RAISE NOTICE '[Migration 000088] Done.'; END $$;
