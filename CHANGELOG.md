# Changelog

All notable changes to Yuheng will be documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the
project follows [Semantic Versioning](https://semver.org/). Yuheng is a 0.x
preview: the `/api/v1` REST API may change between 0.x releases, while MCP tool
names are stable.

## [Unreleased]

This is the state of the first public release. There is no earlier Yuheng
release; the history before this repository belongs to the upstream project
recorded in [`NOTICE`](./NOTICE).

### What the platform includes

- Document ingestion through the `docreader` gRPC parser (PDF, Word, Excel,
  PPT, HTML, EPUB, images, audio, video) with an optional in-process Rust
  parser, plus URL import.
- Data-source connectors with scheduled incremental sync: Feishu/Lark (wiki and
  drive), Notion, Yuque, RSS, GitLab, Tencent ima.
- Hybrid retrieval (vector + BM25) with rerank, query rewrite, FAQ entries, an
  optional Neo4j knowledge graph, and 12 web-search providers (including
  self-hosted SearXNG). Answers stream with citations.
- Auto-Wiki: an LLM pipeline that turns a knowledge base into a Wiki with
  version history, human editing and an issue-feedback loop.
- Multi-tenant workspaces with four roles, organizations and shared spaces,
  audit logs, scoped API keys, AES-256-GCM encryption of stored credentials,
  Langfuse tracing and rate limiting.
- Interfaces for other software: REST API (`/api/v1`, Swagger UI), an MCP server
  with 23 tools, a Go SDK, the `yuheng` CLI and a DeepSeek Harness plugin.
- Optional collaborative documents (`docs` Compose profile).
- Knowledge health: documents of a knowledge base are compared as they
  change — word-for-word copies are reported as duplicates, near-copies that
  differ (one of the two probably out of date) as divergent, with the
  differences marked — and a knowledge base can set a review period after
  which a document nobody has confirmed is due for review. "Not helpful" on an
  answer is taken to the documents it cites. Every problem is routed to a
  person from the documents' owners and recent editors, collected in a
  personal to-do, and settled by confirming a document, superseding one of
  two (a docs page is excluded and marked, not deleted), or dismissing with a
  reason. See `website-docs/03-features/22-knowledge-health.md`.
- A Docker Compose stack (frontend, backend, docreader, PostgreSQL/ParadeDB,
  Redis, RustFS) and a Helm chart. Images are not published; build them locally.

### Storage

- `storage_backends` is the only storage configuration. The deployment's own
  storage (`STORAGE_TYPE`, `S3_*`, `LOCAL_STORAGE_PATH_PREFIX`) is one shared,
  read-only row with id `env`, rewritten from the environment on every start;
  its S3 keys are read from the environment and never stored. The server
  refuses to start when the environment names an unsupported or disallowed
  provider, an incomplete S3 configuration, or a different location while
  files are stored at the old one. New workspaces default to `env` instead of
  getting a copy of it.
- Every file is located by its resource row: the backend it was written to and
  the backend's own locator. Reads never depend on the caller's workspace or a
  knowledge base's current binding, so files on user-registered backends,
  files of shared knowledge bases and docs attachments all resolve the same
  way, and a knowledge base or docs space can be rebound while it has files
  (new files go to the new backend). Workspace defaults, knowledge base and
  docs space bindings are required.
- Writes always name a backend: knowledge base files (including FAQ import
  data) go to the knowledge base's backend; chat images, session attachments
  and temporary documents go to the workspace default.
- Disabling, deleting and un-sharing a backend count its uses in every
  workspace (defaults, knowledge bases, docs spaces, stored files); a shared
  backend can be set as a workspace default.
- File proxies (`/files`, the knowledge-base and message-scoped proxies,
  `/r/<token>`) accept `resource://` references only. With `APP_EXTERNAL_URL`
  set, public file URLs are `/r/<token>` grants for every backend.
- Knowledge base responses carry a `storage_backend` reference (id, name,
  provider, source, is_builtin) for the owning workspace.
- Removed: the workspace `storage-engine-config` KV key and
  `tenants.storage_engine_config` (it held S3 keys unencrypted),
  `knowledge_bases.storage_provider_config`, `GET /system/storage-engine-status`,
  `POST /system/storage-engine-check`, `GET|HEAD /api/v1/files/presigned`, the
  multimodal test's `storage_type` field, the `storage://` path wrapper and
  `STORAGE_TYPE=dummy`. Go SDK: `KnowledgeBase.StorageBackendID` replaces
  `StorageProviderConfig`; CLI: `kb create --storage-backend <id>` replaces
  `--storage-provider`. Migrations `000134`–`000137` target recreated
  deployments and keep no compatibility with data written before them.

### Registration

- Registration is closed by default after the first account. The first person
  to register becomes the administrator of the deployment; after that public
  registration is closed and members join by invitation. `DISABLE_REGISTRATION`
  overrides this: `true` keeps registration closed, `false` keeps it open.

### Removed from the upstream code

- The agent runtime, agent-related settings (skills, tool approvals, MCP
  service management) and the endpoints behind them, which now return `404`.
  Yuheng is the knowledge layer that agents call; it is not an agent.
- IM channel integrations.
- Every vector/search engine except PostgreSQL. Elasticsearch, OpenSearch,
  Milvus, Weaviate, Qdrant, Doris and Tencent Cloud VectorDB drivers are gone;
  PostgreSQL with ParadeDB (`pg_search`) and pgvector is the only database, in
  production and in tests.
- Dedicated cloud-storage providers (MinIO, COS, TOS, OSS, KS3, OBS). File
  storage is now `local` or `s3`; any S3-compatible service, including the
  vendors above through their S3 endpoints, works with `s3`. Existing
  `minio://`, `cos://`, `tos://`, `oss://`, `ks3://` and `obs://` file paths are
  no longer recognized.
- Any mention of the WeChat mini program: no source for it exists in this
  repository.

### Destructive migrations

Some database migrations delete data (for example migration `000089_drop_agent_infra`,
which drops the tables of the removed agent runtime) and cannot be reversed. Back up the database and the file
storage before upgrading; see the
[backup and upgrade guide](./website-docs/01-getting-started/05-backup-and-upgrade.md).
