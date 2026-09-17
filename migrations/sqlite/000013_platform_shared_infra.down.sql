DROP INDEX IF EXISTS idx_web_search_providers_builtin;
DROP INDEX IF EXISTS idx_vector_stores_builtin;
DROP INDEX IF EXISTS idx_storage_backends_builtin;

ALTER TABLE web_search_providers DROP COLUMN is_builtin;
ALTER TABLE vector_stores DROP COLUMN is_builtin;
ALTER TABLE storage_backends DROP COLUMN is_builtin;
