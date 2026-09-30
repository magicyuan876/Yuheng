-- Description: drop wiki_page_issues, a table nothing writes.
--
-- The table held problems flagged on wiki pages by the upstream project's
-- wiki agent (reported_by "wiki-researcher-agent"), which were then fixed by
-- handing them to an agent chat session. Agents were removed from this fork,
-- and with them the only writer; the list and status routes read and edited
-- rows that could no longer appear. The wiki's own checks are the lint report
-- and auto-fix, which compute the problems from the pages each time and store
-- nothing. Any rows still here came from before the fork and name agent work
-- that will never run, so they go with the table.
DO $$ BEGIN RAISE NOTICE '[Migration 000130] Dropping wiki_page_issues...'; END $$;

DROP TABLE IF EXISTS wiki_page_issues;
