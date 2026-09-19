-- Rollback: remove the docs module and tenant groups.
--
-- This drops every document, attachment record and permission the module
-- holds. Attachment bytes in the storage backend are NOT removed by this
-- migration; run the trash purge (or delete the docs/ prefix in the bucket)
-- before rolling back a deployment that was actually used.

DO $$ BEGIN RAISE NOTICE '[Migration 000120 DOWN] Dropping docs module tables...'; END $$;

DROP TABLE IF EXISTS docs_edit_leases;
DROP TABLE IF EXISTS docs_export_jobs;
DROP TABLE IF EXISTS docs_import_jobs;
DROP TABLE IF EXISTS docs_page_labels;
DROP TABLE IF EXISTS docs_labels;
DROP TABLE IF EXISTS docs_templates;
DROP TABLE IF EXISTS docs_transclusion_blocks;
DROP TABLE IF EXISTS docs_links;
DROP TABLE IF EXISTS docs_notifications;
DROP TABLE IF EXISTS docs_watchers;
DROP TABLE IF EXISTS docs_shares;
DROP TABLE IF EXISTS docs_attachments;
DROP TABLE IF EXISTS docs_comments;
DROP TABLE IF EXISTS docs_page_grants;
DROP TABLE IF EXISTS docs_page_access;
DROP TABLE IF EXISTS docs_page_revisions;
DROP TABLE IF EXISTS docs_pages;
DROP TABLE IF EXISTS docs_space_members;
DROP TABLE IF EXISTS docs_spaces;
DROP TABLE IF EXISTS tenant_group_members;
DROP TABLE IF EXISTS tenant_groups;

-- pg_trgm is left installed: other objects may have started using it.

DO $$ BEGIN RAISE NOTICE '[Migration 000120 DOWN] Done.'; END $$;
