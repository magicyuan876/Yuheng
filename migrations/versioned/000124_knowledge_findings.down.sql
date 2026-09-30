DROP TRIGGER IF EXISTS trg_knowledges_forget_findings_on_delete ON knowledges;
DROP TRIGGER IF EXISTS trg_knowledges_forget_findings_on_update ON knowledges;
DROP FUNCTION IF EXISTS knowledge_findings_forget_knowledge();
DROP TABLE IF EXISTS knowledge_finding_scans;
DROP TABLE IF EXISTS knowledge_findings;
