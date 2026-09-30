-- Description: people's feedback on answers — helpful or not, and why.
--
-- One row per person per answer; changing one's mind updates it, taking it
-- back deletes it. A "not helpful" on an answer that cites documents of the
-- workspace is how knowledge health learns that a document may be wrong: the
-- dispute detector counts the down-votes on answers citing a document since
-- somebody last vouched for it, and takes them to its owner.
--
-- The finding is seen by whoever can read the knowledge base, so it carries
-- what the person wrote for it — the comment — and the start of the answer,
-- which the knowledge base produced; the question they asked in their own
-- conversation only when they chose to attach it (share_question).
--
-- tenant_id is the workspace the conversation happened in. Feedback reaches
-- only documents of that same workspace: a question asked in one workspace is
-- not shown to the maintainers of another's shared knowledge base.
DO $$ BEGIN RAISE NOTICE '[Migration 000128] Creating message_feedback...'; END $$;

CREATE TABLE IF NOT EXISTS message_feedback (
    id          VARCHAR(36)  PRIMARY KEY,
    tenant_id   BIGINT       NOT NULL,
    session_id  VARCHAR(36)  NOT NULL,
    message_id  VARCHAR(36)  NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id     VARCHAR(36)  NOT NULL,
    rating      VARCHAR(8)   NOT NULL,
    comment     TEXT         NOT NULL DEFAULT '',
    share_question BOOLEAN   NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT message_feedback_rating_check CHECK (rating IN ('up', 'down'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_message_feedback_person
    ON message_feedback (message_id, user_id);
CREATE INDEX IF NOT EXISTS idx_message_feedback_session
    ON message_feedback (session_id, user_id);
-- The dispute detector reads a workspace's recent down-votes.
CREATE INDEX IF NOT EXISTS idx_message_feedback_disputes
    ON message_feedback (tenant_id, updated_at) WHERE rating = 'down';
