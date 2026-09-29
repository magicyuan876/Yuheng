ALTER TABLE docs_pages ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'published'
    CHECK (status IN ('draft', 'published'));

UPDATE docs_pages SET status = 'draft' WHERE exclude_from_knowledge;

ALTER TABLE docs_pages DROP COLUMN IF EXISTS exclude_from_knowledge;
