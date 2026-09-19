-- The docs module's schema in SQLite, for tests only.
--
-- Postgres is the only database this product runs on; SQLite is what the
-- repository and service tests open in memory, following the convention in
-- CLAUDE.md (GORM's AutoMigrate does not map the `type:jsonb` tags cleanly,
-- so the DDL is written out rather than generated).
--
-- It mirrors versioned migration 000120_docs_module: the online documents
-- module (spaces, pages, collaboration state, permissions, comments,
-- attachments, sharing, notifications, templates, labels, import jobs) and
-- tenant groups. **Changing one means changing the other** — there is no
-- migrator reading this file any more, so nothing else will notice a drift.
--
-- Dialect differences from the Postgres version:
--   * JSONB -> TEXT (JSON text), BYTEA -> BLOB, TIMESTAMPTZ -> DATETIME;
--   * no generated tsvector column and no trigram index: full-text search is
--     a Postgres feature and the tests that need it run against Postgres;
--   * partial unique indexes and CHECK constraints are supported by SQLite
--     and kept identical so both dialects reject the same rows.
--
-- The repository tests execute this file against an in-memory database, so
-- it must stay valid for the stock go-sqlite3 build.

CREATE TABLE IF NOT EXISTS tenant_groups (
    id           VARCHAR(36)  PRIMARY KEY,
    tenant_id    INTEGER      NOT NULL,
    name         VARCHAR(128) NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    is_default   BOOLEAN      NOT NULL DEFAULT 0,
    source       VARCHAR(16)  NOT NULL DEFAULT 'manual',
    external_id  VARCHAR(255),
    creator_id   VARCHAR(36),
    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_tenant_groups_name
    ON tenant_groups (tenant_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_tenant_groups_default
    ON tenant_groups (tenant_id) WHERE is_default = 1 AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS tenant_group_members (
    group_id   VARCHAR(36) NOT NULL REFERENCES tenant_groups(id) ON DELETE CASCADE,
    user_id    VARCHAR(36) NOT NULL,
    tenant_id  INTEGER     NOT NULL,
    added_by   VARCHAR(36),
    created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_tenant_group_members_user
    ON tenant_group_members (tenant_id, user_id);

CREATE TABLE IF NOT EXISTS docs_spaces (
    id                  VARCHAR(36)  PRIMARY KEY,
    tenant_id           INTEGER      NOT NULL,
    slug                VARCHAR(64)  NOT NULL,
    name                VARCHAR(255) NOT NULL,
    description         TEXT         NOT NULL DEFAULT '',
    icon                VARCHAR(64),
    visibility          VARCHAR(16)  NOT NULL DEFAULT 'private'
                        CHECK (visibility IN ('private', 'open', 'public')),
    default_role        VARCHAR(16)  NOT NULL DEFAULT 'none'
                        CHECK (default_role IN ('none', 'reader', 'writer')),
    knowledge_base_id   VARCHAR(36),
    storage_backend_id  VARCHAR(36),
    settings            TEXT         NOT NULL DEFAULT '{}',
    quota_bytes         INTEGER      NOT NULL DEFAULT 0,
    creator_id          VARCHAR(36),
    created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at          DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_docs_spaces_slug
    ON docs_spaces (tenant_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_docs_spaces_tenant ON docs_spaces (tenant_id);
CREATE INDEX IF NOT EXISTS idx_docs_spaces_kb ON docs_spaces (knowledge_base_id);

CREATE TABLE IF NOT EXISTS docs_space_members (
    id              VARCHAR(36) PRIMARY KEY,
    space_id        VARCHAR(36) NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
    tenant_id       INTEGER     NOT NULL,
    principal_type  VARCHAR(8)  NOT NULL CHECK (principal_type IN ('user', 'group')),
    principal_id    VARCHAR(36) NOT NULL,
    role            VARCHAR(16) NOT NULL CHECK (role IN ('admin', 'writer', 'reader')),
    added_by        VARCHAR(36),
    created_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (space_id, principal_type, principal_id)
);
CREATE INDEX IF NOT EXISTS idx_docs_space_members_principal
    ON docs_space_members (tenant_id, principal_type, principal_id);

CREATE TABLE IF NOT EXISTS docs_pages (
    id                  VARCHAR(36)  PRIMARY KEY,
    short_id            VARCHAR(12)  NOT NULL,
    tenant_id           INTEGER      NOT NULL,
    space_id            VARCHAR(36)  NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
    parent_id           VARCHAR(36)  REFERENCES docs_pages(id) ON DELETE CASCADE,
    position            VARCHAR(64)  NOT NULL DEFAULT '',
    title               VARCHAR(512) NOT NULL DEFAULT '',
    icon                VARCHAR(64),
    cover               VARCHAR(1024),
    content             TEXT,
    ydoc                BLOB,
    ydoc_version        INTEGER      NOT NULL DEFAULT 0,
    text_content        TEXT         NOT NULL DEFAULT '',
    status              VARCHAR(16)  NOT NULL DEFAULT 'published'
                        CHECK (status IN ('draft', 'published')),
    is_locked           BOOLEAN      NOT NULL DEFAULT 0,
    template_id         VARCHAR(36),
    source_refs         TEXT         NOT NULL DEFAULT '[]',
    contributor_ids     TEXT         NOT NULL DEFAULT '[]',
    creator_id          VARCHAR(36),
    last_editor_id      VARCHAR(36),
    deleted_by          VARCHAR(36),
    word_count          INTEGER      NOT NULL DEFAULT 0,
    attachment_bytes    INTEGER      NOT NULL DEFAULT 0,
    created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    content_updated_at  DATETIME,
    deleted_at          DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_docs_pages_short_id ON docs_pages (tenant_id, short_id);
CREATE INDEX IF NOT EXISTS idx_docs_pages_tree
    ON docs_pages (space_id, parent_id, position) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_docs_pages_space_updated
    ON docs_pages (tenant_id, space_id, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_docs_pages_trash
    ON docs_pages (space_id, deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS docs_page_revisions (
    id            VARCHAR(36)  PRIMARY KEY,
    page_id       VARCHAR(36)  NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    tenant_id     INTEGER      NOT NULL,
    space_id      VARCHAR(36)  NOT NULL,
    version       INTEGER      NOT NULL,
    title         VARCHAR(512) NOT NULL DEFAULT '',
    icon          VARCHAR(64),
    content       TEXT         NOT NULL,
    text_content  TEXT         NOT NULL DEFAULT '',
    editor_ids    TEXT         NOT NULL DEFAULT '[]',
    reason        VARCHAR(16)  NOT NULL DEFAULT 'interval',
    created_by    VARCHAR(36),
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (page_id, version)
);
CREATE INDEX IF NOT EXISTS idx_docs_page_revisions_page
    ON docs_page_revisions (page_id, created_at DESC);

CREATE TABLE IF NOT EXISTS docs_page_access (
    page_id     VARCHAR(36) PRIMARY KEY REFERENCES docs_pages(id) ON DELETE CASCADE,
    tenant_id   INTEGER     NOT NULL,
    space_id    VARCHAR(36) NOT NULL,
    mode        VARCHAR(16) NOT NULL DEFAULT 'restricted' CHECK (mode IN ('restricted')),
    created_by  VARCHAR(36),
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_docs_page_access_space ON docs_page_access (space_id);

CREATE TABLE IF NOT EXISTS docs_page_grants (
    id              VARCHAR(36) PRIMARY KEY,
    page_id         VARCHAR(36) NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    tenant_id       INTEGER     NOT NULL,
    principal_type  VARCHAR(8)  NOT NULL CHECK (principal_type IN ('user', 'group')),
    principal_id    VARCHAR(36) NOT NULL,
    role            VARCHAR(16) NOT NULL CHECK (role IN ('admin', 'writer', 'reader')),
    added_by        VARCHAR(36),
    created_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (page_id, principal_type, principal_id)
);
CREATE INDEX IF NOT EXISTS idx_docs_page_grants_principal
    ON docs_page_grants (tenant_id, principal_type, principal_id);

CREATE TABLE IF NOT EXISTS docs_comments (
    id           VARCHAR(36) PRIMARY KEY,
    tenant_id    INTEGER     NOT NULL,
    space_id     VARCHAR(36) NOT NULL,
    page_id      VARCHAR(36) NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    parent_id    VARCHAR(36) REFERENCES docs_comments(id) ON DELETE CASCADE,
    body         TEXT        NOT NULL,
    anchor       TEXT,
    quoted_text  TEXT,
    creator_id   VARCHAR(36) NOT NULL,
    resolved_at  DATETIME,
    resolved_by  VARCHAR(36),
    edited_at    DATETIME,
    created_at   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME
);
CREATE INDEX IF NOT EXISTS idx_docs_comments_page ON docs_comments (page_id, created_at);

CREATE TABLE IF NOT EXISTS docs_attachments (
    id           VARCHAR(36)   PRIMARY KEY,
    tenant_id    INTEGER       NOT NULL,
    space_id     VARCHAR(36)   NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
    page_id      VARCHAR(36)   REFERENCES docs_pages(id) ON DELETE SET NULL,
    file_path    VARCHAR(1024) NOT NULL,
    file_name    VARCHAR(512)  NOT NULL,
    file_ext     VARCHAR(32)   NOT NULL DEFAULT '',
    mime         VARCHAR(255)  NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   INTEGER       NOT NULL DEFAULT 0,
    sha256       CHAR(64),
    width        INTEGER,
    height       INTEGER,
    kind         VARCHAR(16)   NOT NULL DEFAULT 'file',
    uploader_id  VARCHAR(36),
    created_at   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME
);
CREATE INDEX IF NOT EXISTS idx_docs_attachments_page ON docs_attachments (page_id);
CREATE INDEX IF NOT EXISTS idx_docs_attachments_space ON docs_attachments (space_id);
CREATE INDEX IF NOT EXISTS idx_docs_attachments_sha ON docs_attachments (tenant_id, sha256);

CREATE TABLE IF NOT EXISTS docs_shares (
    id                  VARCHAR(36)  PRIMARY KEY,
    tenant_id           INTEGER      NOT NULL,
    space_id            VARCHAR(36)  NOT NULL,
    page_id             VARCHAR(36)  NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    key                 VARCHAR(64)  NOT NULL UNIQUE,
    include_children    BOOLEAN      NOT NULL DEFAULT 0,
    allow_search_index  BOOLEAN      NOT NULL DEFAULT 0,
    password_hash       VARCHAR(255),
    expires_at          DATETIME,
    view_count          INTEGER      NOT NULL DEFAULT 0,
    creator_id          VARCHAR(36),
    created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at          DATETIME
);
CREATE INDEX IF NOT EXISTS idx_docs_shares_page ON docs_shares (page_id);

CREATE TABLE IF NOT EXISTS docs_watchers (
    id          VARCHAR(36) PRIMARY KEY,
    tenant_id   INTEGER     NOT NULL,
    user_id     VARCHAR(36) NOT NULL,
    space_id    VARCHAR(36) NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
    page_id     VARCHAR(36) REFERENCES docs_pages(id) ON DELETE CASCADE,
    reason      VARCHAR(16) NOT NULL DEFAULT 'manual',
    muted_at    DATETIME,
    created_by  VARCHAR(36),
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_docs_watchers_page
    ON docs_watchers (user_id, page_id) WHERE page_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uniq_docs_watchers_space
    ON docs_watchers (user_id, space_id) WHERE page_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_docs_watchers_page ON docs_watchers (page_id);

CREATE TABLE IF NOT EXISTS docs_notifications (
    id           VARCHAR(36) PRIMARY KEY,
    tenant_id    INTEGER     NOT NULL,
    user_id      VARCHAR(36) NOT NULL,
    kind         VARCHAR(32) NOT NULL,
    actor_id     VARCHAR(36),
    space_id     VARCHAR(36),
    page_id      VARCHAR(36) REFERENCES docs_pages(id) ON DELETE CASCADE,
    comment_id   VARCHAR(36) REFERENCES docs_comments(id) ON DELETE CASCADE,
    payload      TEXT        NOT NULL DEFAULT '{}',
    read_at      DATETIME,
    emailed_at   DATETIME,
    archived_at  DATETIME,
    created_at   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_docs_notifications_user ON docs_notifications (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_docs_notifications_unread
    ON docs_notifications (user_id) WHERE read_at IS NULL AND archived_at IS NULL;

CREATE TABLE IF NOT EXISTS docs_links (
    tenant_id       INTEGER     NOT NULL,
    source_page_id  VARCHAR(36) NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    target_page_id  VARCHAR(36) NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    kind            VARCHAR(16) NOT NULL DEFAULT 'link',
    created_at      DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_page_id, target_page_id, kind)
);
CREATE INDEX IF NOT EXISTS idx_docs_links_target ON docs_links (target_page_id);

CREATE TABLE IF NOT EXISTS docs_transclusion_blocks (
    id            VARCHAR(36) PRIMARY KEY,
    tenant_id     INTEGER     NOT NULL,
    page_id       VARCHAR(36) NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    block_id      VARCHAR(40) NOT NULL,
    content       TEXT        NOT NULL,
    text_content  TEXT        NOT NULL DEFAULT '',
    updated_at    DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (page_id, block_id)
);

CREATE TABLE IF NOT EXISTS docs_templates (
    id              VARCHAR(36)  PRIMARY KEY,
    tenant_id       INTEGER      NOT NULL,
    space_id        VARCHAR(36)  REFERENCES docs_spaces(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL DEFAULT '',
    icon            VARCHAR(64),
    content         TEXT         NOT NULL,
    text_content    TEXT         NOT NULL DEFAULT '',
    category        VARCHAR(64)  NOT NULL DEFAULT '',
    creator_id      VARCHAR(36),
    last_editor_id  VARCHAR(36),
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME
);
CREATE INDEX IF NOT EXISTS idx_docs_templates_tenant ON docs_templates (tenant_id, space_id);

CREATE TABLE IF NOT EXISTS docs_labels (
    id          VARCHAR(36) PRIMARY KEY,
    tenant_id   INTEGER     NOT NULL,
    space_id    VARCHAR(36) NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
    name        VARCHAR(64) NOT NULL,
    color       VARCHAR(16) NOT NULL DEFAULT 'gray',
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (space_id, name)
);

CREATE TABLE IF NOT EXISTS docs_page_labels (
    page_id     VARCHAR(36) NOT NULL REFERENCES docs_pages(id) ON DELETE CASCADE,
    label_id    VARCHAR(36) NOT NULL REFERENCES docs_labels(id) ON DELETE CASCADE,
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (page_id, label_id)
);
CREATE INDEX IF NOT EXISTS idx_docs_page_labels_label ON docs_page_labels (label_id);

CREATE TABLE IF NOT EXISTS docs_import_jobs (
    id                VARCHAR(36)   PRIMARY KEY,
    tenant_id         INTEGER       NOT NULL,
    space_id          VARCHAR(36)   NOT NULL REFERENCES docs_spaces(id) ON DELETE CASCADE,
    kind              VARCHAR(32)   NOT NULL,
    status            VARCHAR(16)   NOT NULL DEFAULT 'pending',
    source_path       VARCHAR(1024) NOT NULL,
    file_name         VARCHAR(512)  NOT NULL DEFAULT '',
    target_parent_id  VARCHAR(36),
    stats             TEXT          NOT NULL DEFAULT '{}',
    error             TEXT          NOT NULL DEFAULT '',
    created_by        VARCHAR(36),
    created_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at       DATETIME
);
CREATE INDEX IF NOT EXISTS idx_docs_import_jobs_space ON docs_import_jobs (space_id, created_at DESC);

CREATE TABLE IF NOT EXISTS docs_edit_leases (
    page_id     VARCHAR(36) PRIMARY KEY REFERENCES docs_pages(id) ON DELETE CASCADE,
    tenant_id   INTEGER     NOT NULL,
    user_id     VARCHAR(36) NOT NULL,
    session_id  VARCHAR(64) NOT NULL,
    expires_at  DATETIME    NOT NULL,
    created_at  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
