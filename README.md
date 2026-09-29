<p align="center">
    <a href="https://github.com/magicyuan876/yuheng/blob/main/LICENSE">
        <img src="https://img.shields.io/badge/License-MIT-ffffff?labelColor=d4eaf7&color=2e6cc4" alt="License">
    </a>
    <a href="./CHANGELOG.md">
        <img alt="Version" src="https://img.shields.io/badge/version-0.1.0-2e6cc4?labelColor=d4eaf7">
    </a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.26-2e6cc4?labelColor=d4eaf7">
    <img alt="Vue" src="https://img.shields.io/badge/Vue-3-2e6cc4?labelColor=d4eaf7">
    <img alt="Python" src="https://img.shields.io/badge/Python-%3E%3D3.10-2e6cc4?labelColor=d4eaf7">
</p>

<p align="center">
| <b>English</b> | <a href="./README_CN.md"><b>简体中文</b></a> |
</p>

# Yuheng — the knowledge platform for the AI-agent era

Yuheng is an open-source, LLM-powered enterprise knowledge platform. It ingests
documents and data from anywhere, parses and organizes them into searchable
knowledge, answers questions with grounded citations, and turns knowledge bases
into publishable Wiki sites.

Yuheng does not try to be your agent. Starting with 0.1.0 it focuses on one job
and does it well: being the **knowledge layer**. Your AI agents — Claude,
Cursor, custom ReAct loops, anything — come to Yuheng for retrieval, Q&A, and
Wiki content over REST and MCP.

## Why Yuheng

- **Knowledge in, answers out.** Upload documents or connect Feishu/Lark,
  Notion, Yuque, RSS, GitLab and more; Yuheng parses, chunks, indexes, and
  starts answering. No pipeline code to write.
- **Grounded, citable Q&A.** Every answer links back to the exact source
  chunks. Hybrid retrieval + rerank + optional knowledge-graph and web-search
  enrichment keep answers accurate.
- **Auto-Wiki.** A knowledge base can be promoted into a full Wiki site by an
  LLM pipeline — with human review, version history, and an issue-feedback loop.
- **Built for agents first-class.** Everything a human can do in the web UI is
  also a REST endpoint (`/api/v1`, JWT or scoped API keys) and an MCP tool
  (23 tools). Plus a Go SDK, a CLI, and a DeepSeek Harness plugin.
- **Enterprise-ready.** Multi-tenant workspaces, four-tier RBAC, organizations
  and shared spaces, audit logs, AES-256-GCM credential encryption, Langfuse
  tracing, rate limiting.

## Key features

**📥 Ingestion & parsing**
- gRPC document-parsing service (`docreader`): PDF, Word, Excel, PPT, HTML,
  MHTML, EPUB, images, video and more; optional in-process Rust parser (`anydoc`)
- Data-source connectors: Feishu/Lark, Notion, Yuque, RSS, GitLab, Tencent IMA,
  with credential encryption and scheduled incremental sync
- Tree folders, multi-tags, batch operations, chunk-level editing with version
  history, custom metadata

**🔎 Retrieval & Q&A**
- One retrieval engine, done thoroughly: PostgreSQL with ParadeDB (BM25
  full-text) and pgvector (vectors) — no separate search cluster to run
- Hybrid retrieval (vector + BM25/full-text), rerank, query rewrite & expansion
- FAQ entries with bulk import and dedup; knowledge graph (Neo4j, optional);
  built-in web search (9 providers + self-hosted SearXNG)
- Streaming answers with a progress timeline and clickable citations

**📖 Auto-Wiki**
- Four-stage LLM pipeline turns a knowledge base into a Wiki site
- Human editing with version history and rollback; issue reports with a
  closed-loop fix flow; Wiki changes feed back into the knowledge activity stream

**🤖 For your AI agents**
- Complete REST API under `/api/v1` — Swagger UI at `/swagger/index.html`
- Scoped API keys with fine-grained capabilities (retrieve, ingest, manage…)
- [`yuheng-mcp`](./mcp-server/): 23 MCP tools over stdio/SSE/HTTP
- [Go SDK](./client/) and [`yuheng` CLI](./cli/); [DeepSeek Harness plugin](./packages/dsh-yuheng/); WeChat mini program

**🏢 Platform**
- Multi-tenant; four-tier workspace roles; organizations & shared spaces
- Audit log, Langfuse observability, task-queue dashboard, rate limiting

## Architecture

```
┌─────────────┐   REST / SSE   ┌──────────────────────────────┐
│  Web / CLI  │ ◄────────────► │  Go backend (Gin, /api/v1)   │
│  Mini prog. │                │  chat pipeline · RAG · Wiki  │
└─────────────┘                │  async tasks (asynq/Redis)   │
┌─────────────┐   MCP (23)     └───────┬──────────────┬───────┘
│ AI agents   │ ◄───────────────────── │              │ gRPC (TLS+token)
└─────────────┘                        │              ▼
                               ┌───────┴───────┐  ┌────────────┐
                               │  docreader    │  │ PostgreSQL │
                               │  (Python)     │  │ + pgvector │
                               └───────────────┘  └────────────┘
```

For the full picture see the [architecture docs](./website-docs/02-architecture/01-overview.md).

## Getting started

The fastest path is Docker Compose:

```bash
git clone https://github.com/magicyuan876/yuheng.git
cd yuheng
cp .env.example .env          # edit secrets before production use
docker compose pull && docker compose up -d
```

Then open:

| Service | URL |
| --- | --- |
| Web UI | http://localhost |
| API | http://localhost:8080 |
| Swagger UI | http://localhost:8080/swagger/index.html |

Optional Compose profiles add Neo4j, RustFS (S3-compatible object storage) and Langfuse:
`docker compose --profile full up -d`.

Other ways to run:

```bash
make dev-start       # local infra (Postgres, Redis, docreader, Langfuse)
make dev-app         # backend with hot reload (Air)
make dev-frontend    # Vite dev server
```

## Clients & integrations

| Client | Path | Notes |
| --- | --- | --- |
| Web UI | [`frontend/`](./frontend/) | Vue 3 + TDesign |
| CLI | [`cli/`](./cli/) | `yuheng` — scriptable JSON output, multi-profile |
| MCP server | [`mcp-server/`](./mcp-server/) | `pip install yuheng-mcp` — 23 tools |
| Go SDK | [`client/`](./client/) | used by the CLI |
| DeepSeek Harness plugin | [`packages/dsh-yuheng/`](./packages/dsh-yuheng/) | `@magicyuan876/dsh-yuheng` |

## Documentation

- [Product documentation](./website-docs/README.md) — getting started, architecture, features, API reference, clients, development (VitePress)
- [Developer docs](./docs/) — design notes, QA, operations (mixed EN/ZH)
- [Changelog](./CHANGELOG.md)

## Development

```bash
make test             # go test -v ./...
make lint             # golangci-lint
cd frontend && npm run type-check && npm test
cd docreader && uv sync && pytest tests/
cd cli && make build && make test
```

## Security

- All credentials (API keys, data-source tokens, MCP secrets) are encrypted
  with AES-256-GCM at rest.
- Outbound HTTP from data sources and URL import goes through an SSRF-safe
  client with allow-listing.
- Never commit `.env`; replace the placeholder secrets in `.env.example` before
  any real deployment. Internal-network deployment is recommended for
  production — see the [installation & deployment notes](./website-docs/01-getting-started/02-installation.md).

## Attribution

Yuheng is derived in part from the [WeKnora](https://github.com/Tencent/WeKnora)
project, MIT-licensed upstream code is used with notice — see
[`NOTICE`](./NOTICE), [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md), and
[`licenses/upstream-weknora/`](./licenses/upstream-weknora/). <!-- license-check: attribution -->

## License

[MIT](./LICENSE)
