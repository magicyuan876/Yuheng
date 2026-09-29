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
- A Docker Compose stack (frontend, backend, docreader, PostgreSQL/ParadeDB,
  Redis, RustFS) and a Helm chart. Images are not published; build them locally.

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
