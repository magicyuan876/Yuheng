-- Description: drop knowledge_bases.storage_provider_config.
--
-- The column named a provider ("local" or "s3") whose parameters lived in the
-- workspace's storage_engine_config. A knowledge base is bound to a concrete
-- backend by storage_backend_id (required since 000134), and a provider name
-- cannot tell two S3 buckets apart, so the column decides nothing and goes.
DO $$ BEGIN RAISE NOTICE '[Migration 000136] knowledge_bases: dropping storage_provider_config...'; END $$;

ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS storage_provider_config;
