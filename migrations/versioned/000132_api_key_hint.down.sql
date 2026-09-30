-- Migration 000132 Down: drop key_hint.
-- The keys the server emptied out of api_key cannot be restored: only their
-- hashes remain, which is what authentication uses. A server before 000132
-- shows those keys with an empty api_key and otherwise works unchanged.
COMMENT ON COLUMN tenant_api_keys.api_key IS NULL;
ALTER TABLE tenant_api_keys DROP COLUMN IF EXISTS key_hint;
