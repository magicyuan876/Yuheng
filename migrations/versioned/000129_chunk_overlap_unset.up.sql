-- Description: a stored chunk overlap of 0 becomes "not chosen".
--
-- Until now an overlap of 0 could not be set: the field was an int, so an
-- omitted overlap and a deliberate 0 were stored alike, and the chunker read
-- both as "use the default" (80 characters). Now 0 means no overlap and an
-- absent key means the default. A 0 already stored was always split with the
-- default, so it is removed here, which keeps every existing knowledge base
-- and every per-document override splitting exactly as it did. Anyone who
-- wanted no overlap can now set 0 and have it take effect.
DO $$ BEGIN RAISE NOTICE '[Migration 000129] Clearing stored chunk_overlap = 0...'; END $$;

UPDATE knowledge_bases
SET chunking_config = chunking_config - 'chunk_overlap'
WHERE chunking_config -> 'chunk_overlap' = '0'::jsonb;

-- Per-document parse overrides live in the knowledge row's metadata. There an
-- overlap of 0 meant "keep the base's", which an absent key now means.
UPDATE knowledges
SET metadata = metadata #- '{process_overrides,chunking_config,chunk_overlap}'
WHERE metadata #> '{process_overrides,chunking_config,chunk_overlap}' = '0'::jsonb;
