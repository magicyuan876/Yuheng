-- Description: API keys are no longer stored in a recoverable form.
--
-- A key is authenticated by key_hash alone, yet api_key kept the key itself
-- (AES-encrypted when SYSTEM_AES_KEY is set, plain text when it is not), and
-- the key-management API decrypted it into every list and update response.
-- Anyone who could read the table, a backup of it, or an Owner's browser
-- could take every key of the workspace.
--
-- key_hint holds what a person needs to tell keys apart: the first and last
-- few characters, never enough to use one. New keys write only key_hash and
-- key_hint. Existing rows are converted by the server at startup rather than
-- here, because SQL cannot decrypt an encrypted api_key: it derives key_hint
-- (and, for rows migrated from tenants.api_key in 000065, the real key_hash)
-- from the stored key and then empties api_key. The column stays, empty,
-- so a server that has not yet started can still find what to convert.
DO $$ BEGIN RAISE NOTICE '[Migration 000132] Adding tenant_api_keys.key_hint...'; END $$;

ALTER TABLE tenant_api_keys ADD COLUMN IF NOT EXISTS key_hint VARCHAR(32) NOT NULL DEFAULT '';

COMMENT ON COLUMN tenant_api_keys.api_key IS
    'Legacy: the key itself. Emptied by the server at startup once key_hint and key_hash are derived; never written for new keys.';
