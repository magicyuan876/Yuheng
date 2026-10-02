-- Migration: 000139_drop_users_tenant_id
-- Description: a user is a global identity; drop users.tenant_id.
--
-- users.tenant_id was the "home workspace" written at registration, when every
-- registration created a personal workspace. Registration no longer creates a
-- workspace, membership lives only in tenant_members, and the one pointer from
-- a user to a workspace is preferences.last_active_tenant_id, validated against
-- tenant_members on every login. The column, its index and its foreign key go.
--
-- The three system_settings keys below configured behaviour that no longer
-- exists (auto-provisioning a workspace at registration, self-service workspace
-- creation and its per-user cap). The registry treats a missing row as the
-- built-in default, so deleting the rows is safe even while the latter two
-- keys are still registered.
DO $$ BEGIN RAISE NOTICE '[Migration 000139] Dropping users.tenant_id...'; END $$;

ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_tenant;
DROP INDEX IF EXISTS idx_users_tenant_id;
ALTER TABLE users DROP COLUMN IF EXISTS tenant_id;

DELETE FROM system_settings
 WHERE key IN ('auth.default_tenant_mode', 'tenant.self_service_creation_enabled', 'tenant.max_owned_per_user');

DO $$ BEGIN RAISE NOTICE '[Migration 000139] users.tenant_id dropped'; END $$;
