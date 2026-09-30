-- Description: stewardship of knowledge — who maintains an entry, and when a
-- person last worked on it — and the review period of a knowledge base.
--
-- owner_id is the maintainer: the person a problem with the entry is taken to,
-- set to the creator and transferable. It is not a permission; who may read or
-- change the entry is decided where it always was.
--
-- reviewed_at / reviewed_by record the last time a PERSON vouched for the
-- content: confirmed it is still right, or changed it. Re-parsing, re-embedding
-- or moving an entry does not count, which is why updated_at (rewritten by
-- every step of the ingestion pipeline) cannot serve. The review clock of an
-- entry runs from COALESCE(reviewed_at, created_at).
--
-- A docs page keeps its own owner, because a page outlives its mirror entry: a
-- page excluded from the knowledge base loses the entry and must not lose who
-- maintains it. NULL there means the creator. The docs module copies the
-- page's steward onto the mirror entry.
DO $$ BEGIN RAISE NOTICE '[Migration 000125] Adding knowledge stewardship...'; END $$;

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS owner_id    VARCHAR(36);
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS reviewed_by VARCHAR(36);

CREATE INDEX IF NOT EXISTS idx_knowledges_owner
    ON knowledges (tenant_id, owner_id) WHERE deleted_at IS NULL AND owner_id IS NOT NULL;

-- 0 switches periodic review off, which is the default: a review period is a
-- promise somebody has to keep, and it is the knowledge base's owner's to make.
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS review_interval_days INTEGER NOT NULL DEFAULT 0;
DO $$ BEGIN
    ALTER TABLE knowledge_bases ADD CONSTRAINT knowledge_bases_review_interval_check
        CHECK (review_interval_days >= 0 AND review_interval_days <= 3650);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE docs_pages ADD COLUMN IF NOT EXISTS owner_id VARCHAR(36);

-- Existing entries: a page's mirror belongs to the page's creator; an upload
-- to whoever the activity log says created it. Entries neither can account
-- for stay without an owner and are routed by the fallbacks.
UPDATE knowledges k
   SET owner_id = p.creator_id
  FROM docs_pages p
 WHERE p.knowledge_id = k.id AND k.owner_id IS NULL AND p.creator_id IS NOT NULL AND p.creator_id <> '';

UPDATE knowledges k
   SET owner_id = a.actor_user_id
  FROM (SELECT DISTINCT ON (target_id) target_id, actor_user_id
          FROM audit_logs
         WHERE action = 'knowledge.created' AND target_type = 'knowledge' AND actor_user_id <> ''
         ORDER BY target_id, id) a
 WHERE a.target_id = k.id AND k.owner_id IS NULL;
