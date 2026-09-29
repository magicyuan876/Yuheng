-- Description: durable work queue and bookkeeping for mirroring docs pages into
-- their space's knowledge base.
--
-- Until now a page change was remembered in a process's memory for a minute
-- and then synchronised: a restart forgot it, a failed embedding call was never
-- retried, binding a knowledge base to a space indexed nothing until somebody
-- pressed rebuild, and every replica of the application indexed the same page.
-- One row per page now records what is pending (due_at), who is working on it
-- (claimed_until), how often it failed, and a hash of what was last sent, so a
-- page that has not changed is not embedded again.
--
-- A separate narrow table rather than columns on docs_pages: this row is
-- rewritten on every edit and every retry, and docs_pages carries the page body
-- and the collaboration state.
DO $$ BEGIN RAISE NOTICE '[Migration 000122] Creating docs_index_state...'; END $$;

CREATE TABLE IF NOT EXISTS docs_index_state (
    page_id        VARCHAR(36) PRIMARY KEY REFERENCES docs_pages(id) ON DELETE CASCADE,
    tenant_id      BIGINT      NOT NULL,
    -- When the page is next to be synchronised; NULL when nothing is pending.
    due_at         TIMESTAMPTZ,
    -- Bumped by every request to synchronise, so a worker that finishes can tell
    -- whether another request arrived while it was working.
    seq            BIGINT      NOT NULL DEFAULT 0,
    -- While in the future, a worker holds the page; a worker that dies simply
    -- lets the claim expire.
    claimed_until  TIMESTAMPTZ,
    attempts       INT         NOT NULL DEFAULT 0,
    last_error     TEXT        NOT NULL DEFAULT '',
    -- Hash of the title and Markdown last sent to the knowledge base.
    indexed_hash   VARCHAR(64) NOT NULL DEFAULT '',
    -- When the page was last found in step with the knowledge base, whether
    -- or not anything had to be sent.
    indexed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_docs_index_state_due
    ON docs_index_state (due_at) WHERE due_at IS NOT NULL;
