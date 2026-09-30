-- Description: remove favourites of agents.
--
-- Agents were removed from this fork, and "agent" is no longer a type a
-- favourite may have: the favourites API refuses to list, add or remove one.
-- Rows of that type, starred before the removal, point at agents that no
-- longer exist and could never be seen or cleared again, so they are
-- deleted rather than kept as unreachable rows.
DO $$ BEGIN RAISE NOTICE '[Migration 000131] Removing agent favourites...'; END $$;

DELETE FROM user_resource_favorites WHERE resource_type = 'agent';
