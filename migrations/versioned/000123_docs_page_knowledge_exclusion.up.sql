-- Description: replace the draft/published state of a docs page with an explicit
-- "exclude from the knowledge base" flag.
--
-- The state never controlled who could read a page (a draft was readable by
-- everyone who could read the page) and had one real effect that no screen
-- said: a draft was kept out of the space's knowledge base, so out of AI answers.
-- The flag names that effect. New pages take part, as every page did by default;
-- existing drafts keep being excluded.
DO $$ BEGIN RAISE NOTICE '[Migration 000123] Replacing docs_pages.status with exclude_from_knowledge'; END $$;

ALTER TABLE docs_pages ADD COLUMN IF NOT EXISTS exclude_from_knowledge BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE docs_pages SET exclude_from_knowledge = TRUE WHERE status = 'draft';

ALTER TABLE docs_pages DROP COLUMN IF EXISTS status;
