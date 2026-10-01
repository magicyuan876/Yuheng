-- Migration 000135 Down: restore the column, empty. The settings it held are
-- not recoverable from storage_backends.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS storage_engine_config JSONB DEFAULT NULL;
COMMENT ON COLUMN tenants.storage_engine_config IS 'Storage engine parameters for Local, MinIO, COS; used for document/file storage and docreader';
