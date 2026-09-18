-- Rollback of 000030_docs_module (mirror of versioned 000120 DOWN).

DROP TABLE IF EXISTS docs_edit_leases;
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
