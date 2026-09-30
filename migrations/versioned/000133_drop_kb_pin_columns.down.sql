-- Migration 000133 Down: restore the columns, with nothing pinned.
-- Pin state lives in user_kb_pins and is not copied back.
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMP WITH TIME ZONE NULL;
