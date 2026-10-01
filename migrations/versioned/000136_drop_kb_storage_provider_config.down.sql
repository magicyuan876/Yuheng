-- Migration 000136 Down: restore the column, empty. A knowledge base's
-- provider is its storage backend's provider; it is not copied back.
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS storage_provider_config JSONB DEFAULT NULL;
COMMENT ON COLUMN knowledge_bases.storage_provider_config IS 'Storage provider config for this KB. Only stores provider name; credentials come from tenant StorageEngineConfig.';
