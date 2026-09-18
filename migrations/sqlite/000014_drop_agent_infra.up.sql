-- Migration: 000014_drop_agent_infra (sqlite / lite edition)
-- Description: Mirror of versioned 000089 for the embedded SQLite database.
-- The lite edition never had custom_agents / agent_shares / tenant_skills /
-- mcp_tool_approvals / im_channels / embed_channels tables; those DROPs are
-- no-ops here. Agent columns on sessions / messages / message_suggestion_sets
-- are dropped, except sessions.agent_config and messages.agent_steps which are
-- renamed to last_request_state / turn_steps (both are reused by the
-- knowledge-chat pipeline). Data is NOT migrated.

DROP TABLE IF EXISTS agent_shares;
DROP TABLE IF EXISTS tenant_disabled_shared_agents;
DROP TABLE IF EXISTS mcp_tool_approvals;
DROP TABLE IF EXISTS custom_agents;

DROP TABLE IF EXISTS memory_item_embeddings;
DROP TABLE IF EXISTS memory_doc_affinity;
DROP TABLE IF EXISTS memory_topic_stats;
DROP TABLE IF EXISTS memory_tombstones;
DROP TABLE IF EXISTS memory_items;
DROP TABLE IF EXISTS memory_subjects;

DROP TABLE IF EXISTS tenant_skill_snapshots;
DROP TABLE IF EXISTS tenant_skills;

DROP TABLE IF EXISTS im_channel_sessions;
DROP TABLE IF EXISTS im_channels;
DROP TABLE IF EXISTS embed_channels;

DROP INDEX IF EXISTS idx_sessions_agent_id;
ALTER TABLE sessions RENAME COLUMN agent_config TO last_request_state;
ALTER TABLE sessions DROP COLUMN agent_id;

DROP INDEX IF EXISTS idx_messages_agent_id;
ALTER TABLE messages DROP COLUMN agent_id;
ALTER TABLE messages DROP COLUMN agent_tenant_id;
ALTER TABLE messages RENAME COLUMN agent_steps TO turn_steps;
ALTER TABLE messages DROP COLUMN agent_duration_ms;

ALTER TABLE message_suggestion_sets DROP COLUMN agent_id;
ALTER TABLE message_suggestion_sets DROP COLUMN agent_tenant_id;
