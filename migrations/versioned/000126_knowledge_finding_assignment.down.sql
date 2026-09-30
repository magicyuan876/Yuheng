DROP INDEX IF EXISTS idx_knowledge_findings_assignee;
ALTER TABLE knowledge_findings DROP COLUMN IF EXISTS resolution;
ALTER TABLE knowledge_findings DROP COLUMN IF EXISTS assigned_by;
ALTER TABLE knowledge_findings DROP COLUMN IF EXISTS assignee_id;
