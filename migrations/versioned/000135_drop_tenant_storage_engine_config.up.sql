-- Description: drop tenants.storage_engine_config.
--
-- The column was the workspace's storage settings before storage backends
-- existed: a provider name plus local and S3 parameters, S3 keys included, in
-- plain JSON (it was never encrypted, unlike storage_backends.config). Since
-- 000134 every byte is written to a storage_backends row and every file is
-- read through its resource row, so nothing reads this column any more, and
-- the server no longer writes it. It goes rather than lingering as a second,
-- unencrypted copy of credentials.
DO $$ BEGIN RAISE NOTICE '[Migration 000135] tenants: dropping storage_engine_config...'; END $$;

ALTER TABLE tenants DROP COLUMN IF EXISTS storage_engine_config;
