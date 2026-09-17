-- Mirrors versioned migration 000088_platform_shared_infra:
-- platform-shared rows for web search providers, vector stores and storage
-- backends. A row with is_builtin = 1 is visible to every workspace; reads
-- become "(tenant_id = ? OR is_builtin = 1)" while writes stay pinned to the
-- owning tenant and, at the service layer, to system administrators.
--
-- Lite runs one workspace in practice, so sharing is close to a no-op here.
-- The column exists anyway so the same repository queries compile and behave
-- identically on both dialects — a divergence would only surface as a
-- "no such column" at runtime on the path nobody exercises locally.
--
-- tenant_sandbox_configs is deliberately excluded; see the Postgres migration
-- for why (skills are per-tenant but the image they bake into is recorded on
-- the config row).

ALTER TABLE web_search_providers ADD COLUMN is_builtin BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE vector_stores ADD COLUMN is_builtin BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE storage_backends ADD COLUMN is_builtin BOOLEAN NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_web_search_providers_builtin
    ON web_search_providers (is_builtin);
CREATE INDEX IF NOT EXISTS idx_vector_stores_builtin
    ON vector_stores (is_builtin);
CREATE INDEX IF NOT EXISTS idx_storage_backends_builtin
    ON storage_backends (is_builtin);
