-- Description: a docs page superseded by another document.
--
-- Superseding keeps one of two documents and takes the other out of the
-- knowledge base. A page is not deleted for that — it is excluded from the
-- knowledge base, like any page its writers keep out of AI answers — but a
-- reader who opens it should learn that it has been replaced, and by what.
-- superseded_by records that, as a snapshot taken when it happened: the
-- replacement's knowledge entry, title and, when it is a page, the page; who
-- and when. It is a snapshot because the replacement can itself be renamed,
-- moved or deleted later, and the record of the decision must still read.
-- It is cleared when the page is let back into the knowledge base.
DO $$ BEGIN RAISE NOTICE '[Migration 000127] Adding docs_pages.superseded_by...'; END $$;

ALTER TABLE docs_pages ADD COLUMN IF NOT EXISTS superseded_by JSONB;
