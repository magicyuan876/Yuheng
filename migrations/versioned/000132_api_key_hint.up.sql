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
-- key_hint; the key itself is shown once, when it is created.
--
-- api_key goes. Existing keys keep working (they authenticate by key_hash)
-- and simply show no hint. The one kind of row that relied on api_key is a
-- key copied from tenants.api_key in 000065 whose key_hash is still that
-- migration's placeholder: it has never been usable without the column, so
-- it is removed with it.
DO $$ BEGIN RAISE NOTICE '[Migration 000132] tenant_api_keys: key_hint in, api_key out...'; END $$;

ALTER TABLE tenant_api_keys ADD COLUMN IF NOT EXISTS key_hint VARCHAR(32) NOT NULL DEFAULT '';

DELETE FROM tenant_api_keys WHERE key_hash LIKE 'migrated-tenant-%';

ALTER TABLE tenant_api_keys DROP COLUMN IF EXISTS api_key;
