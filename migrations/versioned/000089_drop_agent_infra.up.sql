-- Migration: 000089_drop_agent_infra
-- Description: Remove the agent runtime infrastructure. The project is a
-- knowledge-base platform now (RAG QA + auto-wiki + retrieval APIs); the
-- ReAct agent engine, sandbox, skills, IM/embed channels and long-term
-- memory were removed from the code, and this migration drops their tables
-- and columns. Data in these tables is NOT migrated (agent sessions have no
-- knowledge-platform equivalent).

DO $$ BEGIN RAISE NOTICE '[Migration 000089] Dropping agent infrastructure...'; END $$;

-- Custom agents and sharing (depends on custom_agents)
DROP TABLE IF EXISTS agent_shares;
DROP TABLE IF EXISTS tenant_disabled_shared_agents;
DROP TABLE IF EXISTS mcp_tool_approvals;
DROP TABLE IF EXISTS custom_agents;

-- Agent long-term memory (children before parents where FKs exist)
DROP TABLE IF EXISTS memory_item_embeddings;
DROP TABLE IF EXISTS memory_doc_affinity;
DROP TABLE IF EXISTS memory_topic_stats;
DROP TABLE IF EXISTS memory_tombstones;
DROP TABLE IF EXISTS memory_items;
DROP TABLE IF EXISTS memory_subjects;

-- Tenant skill subsystem
DROP TABLE IF EXISTS tenant_skill_snapshots;
DROP TABLE IF EXISTS tenant_skills;

-- IM / embed channels (agent publishing surfaces)
DROP TABLE IF EXISTS im_channel_sessions;
DROP TABLE IF EXISTS im_channels;
DROP TABLE IF EXISTS embed_channels;

-- Agent columns on tables that survive. sessions.agent_config and
-- messages.agent_steps are RENAMED rather than dropped: the knowledge-chat
-- pipeline reuses both columns — sessions.last_request_state stores the
-- input-bar request state, messages.turn_steps stores the chat turn steps
-- (thinking / tool calls) shown in the timeline.
DROP INDEX IF EXISTS idx_sessions_agent_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS agent_id;
ALTER TABLE sessions RENAME COLUMN agent_config TO last_request_state;

DROP INDEX IF EXISTS idx_messages_agent_id;
ALTER TABLE messages DROP COLUMN IF EXISTS agent_id;
ALTER TABLE messages DROP COLUMN IF EXISTS agent_tenant_id;
ALTER TABLE messages RENAME COLUMN agent_steps TO turn_steps;
ALTER TABLE messages DROP COLUMN IF EXISTS agent_duration_ms;

ALTER TABLE message_suggestion_sets DROP COLUMN IF EXISTS agent_id;
ALTER TABLE message_suggestion_sets DROP COLUMN IF EXISTS agent_tenant_id;

DO $$ BEGIN RAISE NOTICE '[Migration 000089] Agent infrastructure dropped'; END $$;
