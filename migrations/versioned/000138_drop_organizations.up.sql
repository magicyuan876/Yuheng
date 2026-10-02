-- Migration: 000138_drop_organizations
-- Description: remove organizations and cross-workspace knowledge base sharing.
--
-- A knowledge base is reachable only from the workspace that owns it. The
-- organization layer (000012, 000045, 000046) let workspaces federate and share
-- knowledge bases into the federation; the code that read these tables is
-- gone, so the tables go with it. Share grants, memberships and join requests
-- are NOT migrated: there is no equivalent in the one-enterprise-one-workspace
-- model. The parked organization_members_pre_plan3 table (renamed, never
-- dropped, by 000045) goes at the same time.
--
-- Drop order follows the foreign keys: every child table references
-- organizations(id), so organizations is last.
DO $$ BEGIN RAISE NOTICE '[Migration 000138] Dropping organizations and knowledge base shares...'; END $$;

DROP TABLE IF EXISTS kb_shares;
DROP TABLE IF EXISTS organization_join_requests;
DROP TABLE IF EXISTS organization_tenant_members;
DROP TABLE IF EXISTS organization_members_pre_plan3;
DROP TABLE IF EXISTS organizations;

DO $$ BEGIN RAISE NOTICE '[Migration 000138] Organizations dropped'; END $$;
