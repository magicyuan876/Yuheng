-- Description: who a knowledge-health finding is taken to, and why a closed
-- one was closed.
--
-- assignee_id is routed by every check from the stewardship of the documents
-- involved (see findings.AssignRule); assigned_by is set when a person chose
-- the assignee instead, and a check then leaves the assignment alone.
-- resolution records why a finding was closed: a dismissal names its reason
-- (distinct_scope, intentional), a finding the detectors stopped reporting is
-- cleared. It is an open string, like type, so that detectors added later
-- need no schema change.
DO $$ BEGIN RAISE NOTICE '[Migration 000126] Adding finding assignment...'; END $$;

ALTER TABLE knowledge_findings ADD COLUMN IF NOT EXISTS assignee_id VARCHAR(36);
ALTER TABLE knowledge_findings ADD COLUMN IF NOT EXISTS assigned_by VARCHAR(36);
ALTER TABLE knowledge_findings ADD COLUMN IF NOT EXISTS resolution  VARCHAR(32);

-- "My problems to deal with", across the knowledge bases of a workspace.
CREATE INDEX IF NOT EXISTS idx_knowledge_findings_assignee
    ON knowledge_findings (tenant_id, assignee_id, status) WHERE assignee_id IS NOT NULL;

-- Findings closed before resolutions existed: a system resolution cleared,
-- a dismissal of unknown reason left NULL.
UPDATE knowledge_findings SET resolution = 'cleared'
 WHERE status = 'resolved' AND resolved_by = 'system' AND resolution IS NULL;
