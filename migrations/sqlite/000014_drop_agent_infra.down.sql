-- Migration: 000014_drop_agent_infra (down, sqlite / lite edition)
-- Description: Best-effort structural rollback. Recreates the agent tables
-- and columns as EMPTY skeletons. Data dropped by the up migration is NOT
-- restored. The lite schema is hand-maintained, so these skeletons are
-- intentionally minimal.

CREATE TABLE IF NOT EXISTS custom_agents (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    avatar TEXT NOT NULL DEFAULT '',
    agent_mode VARCHAR(32) NOT NULL DEFAULT 'smart-reasoning',
    config TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS agent_shares (
    id VARCHAR(36) PRIMARY KEY,
    agent_id VARCHAR(36) NOT NULL,
    organization_id VARCHAR(36) NOT NULL,
    permission VARCHAR(32) NOT NULL DEFAULT 'viewer',
    created_at DATETIME
);

CREATE TABLE IF NOT EXISTS tenant_disabled_shared_agents (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    agent_id VARCHAR(36) NOT NULL,
    created_at DATETIME
);

CREATE TABLE IF NOT EXISTS mcp_tool_approvals (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    tool_name VARCHAR(255) NOT NULL DEFAULT '',
    arguments TEXT NOT NULL DEFAULT '{}',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    resolution TEXT NOT NULL DEFAULT '',
    created_at DATETIME,
    resolved_at DATETIME
);

CREATE TABLE IF NOT EXISTS memory_subjects (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    kind VARCHAR(32) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    created_at DATETIME,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS memory_items (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(36) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    source_message_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at DATETIME
);

CREATE TABLE IF NOT EXISTS memory_tombstones (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    reason VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME
);

CREATE TABLE IF NOT EXISTS memory_topic_stats (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    topic VARCHAR(255) NOT NULL DEFAULT '',
    stat TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS memory_doc_affinity (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    score REAL NOT NULL DEFAULT 0,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS memory_item_embeddings (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    embedding BLOB,
    created_at DATETIME
);

CREATE TABLE IF NOT EXISTS tenant_skills (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    source VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT '',
    snapshot_id VARCHAR(36) NOT NULL DEFAULT '',
    install_session_id VARCHAR(36) NOT NULL DEFAULT '',
    install_message_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at DATETIME,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS tenant_skill_snapshots (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    skill_id VARCHAR(36) NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0,
    manifest TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME
);

CREATE TABLE IF NOT EXISTS im_channels (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    agent_id VARCHAR(36) DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS im_channel_sessions (
    id VARCHAR(36) PRIMARY KEY,
    channel_id VARCHAR(36) NOT NULL,
    external_thread_id VARCHAR(255) NOT NULL DEFAULT '',
    agent_id VARCHAR(36) DEFAULT '',
    session_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at DATETIME,
    updated_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_im_channels_agent ON im_channels (agent_id);

CREATE TABLE IF NOT EXISTS embed_channels (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    name VARCHAR(255) NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME,
    updated_at DATETIME
);

ALTER TABLE sessions RENAME COLUMN last_request_state TO agent_config;
ALTER TABLE sessions ADD COLUMN agent_config TEXT DEFAULT NULL;
ALTER TABLE sessions ADD COLUMN agent_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);

ALTER TABLE messages ADD COLUMN agent_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN agent_tenant_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages RENAME COLUMN turn_steps TO agent_steps;
ALTER TABLE messages ADD COLUMN agent_steps TEXT DEFAULT NULL;
ALTER TABLE messages ADD COLUMN agent_duration_ms INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_messages_agent_id ON messages(agent_id);

ALTER TABLE message_suggestion_sets ADD COLUMN agent_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE message_suggestion_sets ADD COLUMN agent_tenant_id INTEGER NOT NULL DEFAULT 0;
