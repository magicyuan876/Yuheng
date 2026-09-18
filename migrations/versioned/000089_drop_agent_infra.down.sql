-- Migration: 000089_drop_agent_infra (down)
-- Description: Best-effort structural rollback for 000089. Recreates the
-- agent tables and columns as EMPTY skeletons (copied from the original
-- migrations 000006/000012/000042/000084/000086/000021/000060). Data dropped
-- by the up migration is NOT restored.

DO $$ BEGIN RAISE NOTICE '[Migration 000089-down] Recreating agent infrastructure skeletons...'; END $$;

CREATE TABLE IF NOT EXISTS custom_agents (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    avatar TEXT NOT NULL DEFAULT '',
    agent_mode VARCHAR(32) NOT NULL DEFAULT 'smart-reasoning',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agent_shares (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id VARCHAR(36) NOT NULL REFERENCES custom_agents(id) ON DELETE CASCADE,
    organization_id VARCHAR(36) NOT NULL,
    permission VARCHAR(32) NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenant_disabled_shared_agents (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mcp_tool_approvals (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    tool_name VARCHAR(255) NOT NULL DEFAULT '',
    arguments JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    resolution TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_subjects (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    kind VARCHAR(32) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_items (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(36) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    source_message_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_tombstones (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    reason VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_topic_stats (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    topic VARCHAR(255) NOT NULL DEFAULT '',
    stat JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_doc_affinity (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL DEFAULT '',
    knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_item_embeddings (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    embedding vector(1024),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenant_skills (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL DEFAULT '',
    source VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT '',
    snapshot_id VARCHAR(36) NOT NULL DEFAULT '',
    install_session_id VARCHAR(36) NOT NULL DEFAULT '',
    install_message_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenant_skill_snapshots (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    skill_id VARCHAR(36) NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0,
    manifest JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS im_channels (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id VARCHAR(36) DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS im_channel_sessions (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id VARCHAR(36) NOT NULL REFERENCES im_channels(id) ON DELETE CASCADE,
    external_thread_id VARCHAR(255) NOT NULL DEFAULT '',
    agent_id VARCHAR(36) DEFAULT '',
    session_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_im_channels_agent ON im_channels (agent_id);

CREATE TABLE IF NOT EXISTS embed_channels (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    name VARCHAR(255) NOT NULL DEFAULT '',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS agent_id VARCHAR(36);
ALTER TABLE sessions RENAME COLUMN last_request_state TO agent_config;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS agent_config JSONB DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);

ALTER TABLE messages ADD COLUMN IF NOT EXISTS agent_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS agent_tenant_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages RENAME COLUMN turn_steps TO agent_steps;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS agent_steps JSONB DEFAULT NULL;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS agent_duration_ms BIGINT DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_messages_agent_id ON messages(agent_id);

ALTER TABLE message_suggestion_sets ADD COLUMN IF NOT EXISTS agent_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE message_suggestion_sets ADD COLUMN IF NOT EXISTS agent_tenant_id INTEGER NOT NULL DEFAULT 0;

DO $$ BEGIN RAISE NOTICE '[Migration 000089-down] Skeletons recreated (empty)'; END $$;
