-- Description: Drop knowledge_bases.cos_config.
-- The column held the storage provider of a knowledge base together with the
-- credentials of the one cloud that used to be supported (Tencent COS). The
-- provider has lived in storage_provider_config since migration 000014 and the
-- credentials in the tenant's storage engine config; the column was only kept
-- as a dual-write. Copy the provider across once more for any row written since
-- that did not fill the new column, then drop it, so no stale credential stays
-- in the table.
DO $$ BEGIN RAISE NOTICE '[Migration 000121] Dropping knowledge_bases.cos_config'; END $$;

UPDATE knowledge_bases
SET storage_provider_config = jsonb_build_object('provider', cos_config->>'provider')
WHERE cos_config->>'provider' IN ('local', 's3')
  AND (storage_provider_config IS NULL
       OR storage_provider_config->>'provider' IS NULL
       OR storage_provider_config->>'provider' = '');

ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS cos_config;
