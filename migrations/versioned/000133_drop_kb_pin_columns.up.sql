-- Description: drop the tenant-wide pin columns from knowledge_bases.
--
-- Pinning a knowledge base is a per-user choice kept in user_kb_pins since
-- 000050, which left knowledge_bases.is_pinned and pinned_at in place so that
-- release could be rolled back. Nothing reads or writes them: the KB model
-- maps both fields to gorm:"-" and fills them from user_kb_pins. They go, so
-- the table no longer carries a second, stale answer to "is this pinned?".
DO $$ BEGIN RAISE NOTICE '[Migration 000133] knowledge_bases: dropping is_pinned / pinned_at...'; END $$;

ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS is_pinned;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS pinned_at;
