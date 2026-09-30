-- Description: knowledge health — problems found by comparing the documents of
-- a knowledge base with each other (near-duplicates first), and when each
-- document was last checked.
--
-- A finding is keyed by a fingerprint that does not depend on which of its
-- documents triggered the check, so re-running a detector over unchanged
-- content updates the same row. type and detector are open strings: detectors
-- added later need no schema change. details carries the evidence (passage
-- pairs, overlap ratio, a hash of what the finding rests on).
--
-- Findings must never outlive the documents they name. Knowledge rows are
-- soft-deleted and moved between knowledge bases by several code paths (single
-- and batch delete, knowledge-base deletion, move), so rather than hooking each
-- of them the database forgets a document's findings itself, in a trigger on
-- knowledges. The reads also join the live documents, which covers the moment
-- between the two.
DO $$ BEGIN RAISE NOTICE '[Migration 000124] Creating knowledge_findings...'; END $$;

CREATE TABLE IF NOT EXISTS knowledge_findings (
    id                   VARCHAR(36)  PRIMARY KEY,
    tenant_id            BIGINT       NOT NULL,
    knowledge_base_id    VARCHAR(36)  NOT NULL,
    type                 VARCHAR(32)  NOT NULL,
    detector             VARCHAR(64)  NOT NULL,
    severity             VARCHAR(16)  NOT NULL,
    status               VARCHAR(16)  NOT NULL DEFAULT 'open',
    fingerprint          VARCHAR(128) NOT NULL,
    subject_knowledge_id VARCHAR(36)  NOT NULL,
    -- NULL for a finding about one document alone.
    related_knowledge_id VARCHAR(36),
    score                REAL,
    details              JSONB        NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- Who closed the finding and when: a user ID for a dismissal, 'system'
    -- for a finding the detector stopped reporting. NULL while open.
    resolved_at          TIMESTAMPTZ,
    resolved_by          VARCHAR(64),
    CONSTRAINT knowledge_findings_status_check CHECK (status IN ('open', 'dismissed', 'resolved')),
    CONSTRAINT knowledge_findings_severity_check CHECK (severity IN ('info', 'warning', 'error'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_knowledge_findings_fingerprint
    ON knowledge_findings (tenant_id, fingerprint);
CREATE INDEX IF NOT EXISTS idx_knowledge_findings_kb_status
    ON knowledge_findings (tenant_id, knowledge_base_id, status);
CREATE INDEX IF NOT EXISTS idx_knowledge_findings_subject
    ON knowledge_findings (subject_knowledge_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_findings_related
    ON knowledge_findings (related_knowledge_id);

-- One row per checked document: when the detectors last ran over it. Kept
-- apart from knowledges, which is rewritten on every status change of the
-- ingestion pipeline.
CREATE TABLE IF NOT EXISTS knowledge_finding_scans (
    knowledge_id      VARCHAR(36) PRIMARY KEY,
    tenant_id         BIGINT      NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    scanned_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_finding_scans_kb
    ON knowledge_finding_scans (tenant_id, knowledge_base_id, scanned_at);

CREATE OR REPLACE FUNCTION knowledge_findings_forget_knowledge()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM knowledge_findings
     WHERE subject_knowledge_id = OLD.id OR related_knowledge_id = OLD.id;
    DELETE FROM knowledge_finding_scans WHERE knowledge_id = OLD.id;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Soft delete and move are updates. The WHEN clause keeps the trigger out of
-- the many status updates the ingestion pipeline makes to the same rows.
DROP TRIGGER IF EXISTS trg_knowledges_forget_findings_on_update ON knowledges;
CREATE TRIGGER trg_knowledges_forget_findings_on_update
    AFTER UPDATE OF deleted_at, knowledge_base_id ON knowledges
    FOR EACH ROW
    WHEN ((OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
          OR OLD.knowledge_base_id IS DISTINCT FROM NEW.knowledge_base_id)
    EXECUTE FUNCTION knowledge_findings_forget_knowledge();

DROP TRIGGER IF EXISTS trg_knowledges_forget_findings_on_delete ON knowledges;
CREATE TRIGGER trg_knowledges_forget_findings_on_delete
    AFTER DELETE ON knowledges
    FOR EACH ROW
    EXECUTE FUNCTION knowledge_findings_forget_knowledge();
