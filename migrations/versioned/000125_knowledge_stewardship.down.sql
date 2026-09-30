ALTER TABLE docs_pages DROP COLUMN IF EXISTS owner_id;
ALTER TABLE knowledge_bases DROP CONSTRAINT IF EXISTS knowledge_bases_review_interval_check;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS review_interval_days;
DROP INDEX IF EXISTS idx_knowledges_owner;
ALTER TABLE knowledges DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE knowledges DROP COLUMN IF EXISTS reviewed_at;
ALTER TABLE knowledges DROP COLUMN IF EXISTS owner_id;
