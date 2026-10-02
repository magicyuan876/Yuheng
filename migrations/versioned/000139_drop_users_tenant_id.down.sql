-- Migration: 000139_drop_users_tenant_id (rollback)
-- Description: recreate users.tenant_id as 000001 defined it (INTEGER, nullable,
-- indexed, foreign key to tenants with ON DELETE SET NULL, comment).
--
-- The column is backfilled from each user's earliest active membership, which
-- is what the code it is being restored for would have resolved as the "home"
-- workspace. The deleted system_settings rows are not restored: their absence
-- means the built-in defaults, which is also what a fresh install has.
DO $$ BEGIN RAISE NOTICE '[Migration 000139] Restoring users.tenant_id...'; END $$;

ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id INTEGER;
COMMENT ON COLUMN users.tenant_id IS 'Tenant ID that the user belongs to';
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_users_tenant') THEN
        ALTER TABLE users ADD CONSTRAINT fk_users_tenant
            FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE SET NULL;
    END IF;
END $$;

UPDATE users u
   SET tenant_id = m.tenant_id
  FROM (
        SELECT DISTINCT ON (user_id) user_id, tenant_id
          FROM tenant_members
         WHERE deleted_at IS NULL AND status = 'active'
         ORDER BY user_id, joined_at ASC, id ASC
       ) m
 WHERE m.user_id = u.id;

DO $$ BEGIN RAISE NOTICE '[Migration 000139] users.tenant_id restored'; END $$;
