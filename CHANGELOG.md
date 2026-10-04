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
- Workspaces with four member roles and workspace groups, system
  administrators who manage workspaces and accounts, audit logs, scoped API
  keys, AES-256-GCM encryption of stored credentials, Langfuse tracing and rate
  limiting.
- Interfaces for other software: REST API (`/api/v1`, Swagger UI), an MCP server
  with 22 tools, a Go SDK, the `yuheng` CLI and a DeepSeek Harness plugin.
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
  knowledge base's current binding, so files on user-registered backends
  and docs attachments all resolve the same way, and a knowledge base or docs space can be rebound while it has files
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

### Docs spaces and knowledge bases

A docs space syncs its pages into a knowledge base, and the web UI now says
which one and lets a space administrator change it.

- Creating a space defaults to creating a knowledge base with it: a document
  knowledge base named like the space, on the space's storage backend, with
  the workspace's default models and the creator as its owner. The form can
  also bind an existing knowledge base or not sync at all; without an
  embedding model in the workspace, creating one is unavailable and the form
  says why.
- Space settings name the bound knowledge base and link to it, and rebind,
  create or unbind it, stating what happens to the pages already synced.
- API: `POST /docs/spaces` and `PUT /docs/spaces/:sid/knowledge-base` take
  `knowledge_base: {"mode": "none" | "existing" | "create", "id"}`; the
  `knowledge_base_id` field they took before is refused with 400. Error code
  `2300` means the workspace has no embedding model.
- Binding an existing knowledge base requires the right to add documents to
  it (its creator or a workspace Owner/Admin), not only to see it, and only
  document knowledge bases can be bound.
- Fixed: rebinding, unbinding or trashing a space left its pages answerable
  from the old knowledge base until the five-minutely sweep reached them, a
  few hundred pages per round; every page of the space is now queued at once.

### Workspace model

A user is a global identity and a company is one workspace with any number of
knowledge bases in it. This replaces the earlier shape, in which every
registration created a personal workspace and workspaces federated through
organizations. Breaking for anyone who deployed a pre-release build:

- Registration creates an account and nothing else. The first person to
  register (bootstrap) creates the deployment's default workspace, optionally
  named through `workspace_name` on `POST /auth/register`, and becomes its
  Owner and the system administrator, in one transaction; after that public
  registration is closed and accounts enter a workspace by invitation or by a
  system administrator. `DISABLE_REGISTRATION` overrides the mode: `true` keeps
  registration closed, `false` keeps it open (accounts registered that way
  belong to no workspace until added). OIDC first login also only creates an
  account.
- Removed: the `create_personal` / `tenantless` provisioning modes,
  `auth.default_tenant_mode` and `YUHENG_AUTH_DEFAULT_TENANT_MODE`,
  self-service workspace creation (`tenant.self_service_creation_enabled`,
  `YUHENG_TENANT_SELF_SERVICE_CREATION_ENABLED`, `tenant.max_owned_per_user`,
  `YUHENG_TENANT_MAX_OWNED_PER_USER`; error code 2005 is retired), the
  `users.tenant_id` "home workspace" column (migration `000139`), the orphan
  self-heal that promoted whoever logged in to Owner of a memberless workspace,
  and the `manage_spaces` API-key capability.
- Organizations and cross-workspace knowledge base sharing are gone (migration
  `000138` drops `organizations`, `organization_tenant_members`,
  `organization_join_requests`, `kb_shares` and the parked
  `organization_members_pre_plan3`; share grants are not migrated). A knowledge
  base is reachable only from the workspace that owns it: another workspace's
  base answers `404`, the same as a missing id. Gone with them: the
  `/organizations/**`, `/shared-knowledge-bases` and
  `/knowledge-bases/:id/shares` routes, `share_count` and `my_permission` on
  knowledge base responses, the `vector_store_source=shared` value, the
  `kb.share_*` audit actions (historical rows render as their raw string), the
  `organizations` deployment capability, the Go SDK's organization client and
  the MCP `list_shared_knowledge_bases` tool (22 tools remain).
- Which workspace a request acts in is resolved in one order everywhere:
  `X-Tenant-ID` header, JWT `tenant_id`, `preferences.last_active_tenant_id`
  (if still an active membership), the earliest active membership, none. A
  user with no workspace can sign in and sees "your account is not in any
  workspace yet"; non-identity routes answer `409 TENANT_REQUIRED`. `/auth/me`
  and `AuthUser` in the Go SDK carry no `tenant_id`.
- `POST /tenants` is a catalog operation for system administrators,
  cross-tenant superusers and platform API keys (`system_tenants_manage`). It
  takes `owner_email` and writes the Owner membership in the same request
  (platform keys must name one; `400`, code 2006). `DELETE /tenants/:id` is
  refused while other members remain (`409`, code 2007) or for the deployment's
  last workspace (`409`, code 2008). `/tenants/all` and `/tenants/search` are
  open to system administrators and return `member_count`. Audit:
  `system.tenant_created`, `system.tenant_deleted`.
- System administrators manage workspaces and members: a "Users & workspaces"
  settings page, `GET/POST /system/admin/tenants/:id/members`,
  `PATCH/DELETE /system/admin/tenants/:id/members/:user_id` (member audit rows
  record `actor_role=system_admin`), and `POST /system/admin/users/create`
  accepts `tenant_id` and `role` to place the new account in a workspace.
- Workspace groups (`tenant_groups`) are a core concept, no longer gated by
  the docs module: `/api/v1/groups/**` is always registered, the API-key
  capability is `manage_members` instead of `docs_admin`, the UI says
  "workspace groups", and the audit actions are `rbac.group_*` (were
  `docs.group.*`). Paths, table and the ACL subject `group:<id>` are unchanged.
- API keys always act as the synthetic `system-<tenantID>` identity instead of
  impersonating the workspace's oldest human member.
- `tenant.default_storage_quota_gb` / `YUHENG_TENANT_DEFAULT_STORAGE_QUOTA_GB`
  defaults to `0` (unlimited); the model's former 10 GB column default is gone.

### Removed from the upstream code

- The agent runtime, agent-related settings (skills, tool approvals, MCP
  service management) and the endpoints behind them, which now return `404`.
  Yuheng is the knowledge layer that agents call; it is not an agent.
- IM channel integrations.
- Every vector/search engine except PostgreSQL. Elasticsearch, OpenSearch,
  Milvus, Weaviate, Qdrant, Doris and Tencent Cloud VectorDB drivers are gone;
  PostgreSQL with ParadeDB (`pg_search`) and pgvector is the only database, in
  production and in tests.
- Organizations and cross-workspace knowledge base sharing (see "Workspace
  model" above).
- Dedicated cloud-storage providers (MinIO, COS, TOS, OSS, KS3, OBS). File
  storage is now `local` or `s3`; any S3-compatible service, including the
  vendors above through their S3 endpoints, works with `s3`. Existing
  `minio://`, `cos://`, `tos://`, `oss://`, `ks3://` and `obs://` file paths are
  no longer recognized.
- Any mention of the WeChat mini program: no source for it exists in this
  repository.

### Destructive migrations

Some database migrations delete data (for example migration `000089_drop_agent_infra`,
which drops the tables of the removed agent runtime, `000138_drop_organizations`,
which drops organizations and knowledge base shares, and
`000139_drop_users_tenant_id`) and cannot be reversed. Back up the database and the file
storage before upgrading; see the
[backup and upgrade guide](./website-docs/01-getting-started/05-backup-and-upgrade.md).
