-- Rollback: restore the column with the provider copied back. The credentials it
-- held are not recoverable, and nothing reads them.
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS cos_config JSONB NOT NULL DEFAULT '{}';

UPDATE knowledge_bases
SET cos_config = jsonb_build_object('provider', storage_provider_config->>'provider')
WHERE storage_provider_config->>'provider' IN ('local', 's3');
