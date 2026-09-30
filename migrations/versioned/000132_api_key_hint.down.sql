-- Migration 000132 Down: restore the api_key column, empty, and drop key_hint.
-- The keys cannot be restored: only their hashes were kept, and hashes are
-- what authentication uses. The placeholder rows deleted on the way up could
-- never authenticate and are not recreated.
ALTER TABLE tenant_api_keys ADD COLUMN IF NOT EXISTS api_key TEXT NOT NULL DEFAULT '';
ALTER TABLE tenant_api_keys DROP COLUMN IF EXISTS key_hint;
