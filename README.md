<p align="center">
    <a href="https://github.com/magicyuan876/Yuheng/blob/main/LICENSE">
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

## Status

Yuheng is a 0.x preview, developed by one maintainer.

- The REST API under `/api/v1` may still change between 0.x releases. The MCP
  tool names are stable: they will not be renamed.
- Most product documentation is currently in Chinese. English translations of
  the [quick start](./website-docs/en/01-getting-started/03-quickstart.md) and the
  [installation guide](./website-docs/en/01-getting-started/02-installation.md) exist;
  everything else under [`website-docs/`](./website-docs/) is Chinese-first.
- The first release does not publish Docker images. You build them locally
  (see [Getting started](#getting-started)).

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
  built-in web search (12 providers, including self-hosted SearXNG)
- Streaming answers with a progress timeline and clickable citations

**📖 Auto-Wiki**
- Four-stage LLM pipeline turns a knowledge base into a Wiki site
- Human editing with version history and rollback; issue reports with a
  closed-loop fix flow; Wiki changes feed back into the knowledge activity stream

**🤖 For your AI agents**
- Complete REST API under `/api/v1` — Swagger UI at `/swagger/index.html`
- Scoped API keys with fine-grained capabilities (retrieve, ingest, manage…)
- [`yuheng-mcp`](./mcp-server/): 23 MCP tools over stdio/SSE/HTTP (install from source)
- [Go SDK](./client/) and [`yuheng` CLI](./cli/); [DeepSeek Harness plugin](./packages/dsh-yuheng/)

**🏢 Platform**
- Multi-tenant; four-tier workspace roles; organizations & shared spaces
- Audit log, Langfuse observability, task-queue dashboard, rate limiting

## Architecture

```
┌─────────────┐   REST / SSE   ┌──────────────────────────────┐
│  Web / CLI  │ ◄────────────► │  Go backend (Gin, /api/v1)   │
│  Go SDK     │                │  chat pipeline · RAG · Wiki  │
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

There are no prebuilt images for this release, so Docker Compose builds them
from the checkout. You need Docker with Compose v2, Node.js and npm (the
frontend is built on the host first), and `git`. Plan for 4 CPU cores and 8 GB
of RAM; the first build downloads a lot and takes a while.

```bash
git clone https://github.com/magicyuan876/Yuheng.git
cd Yuheng
cp .env.example .env
```

Edit `.env` before the first start. The example values for `JWT_SECRET` and
`SYSTEM_AES_KEY` are public, so replace them. The server refuses to start with
an empty or example value:

```bash
openssl rand -hex 32     # -> JWT_SECRET
openssl rand -hex 16     # -> SYSTEM_AES_KEY (32 hex characters = the 32 bytes AES-256 needs)
```

Also change `DB_PASSWORD` and `REDIS_PASSWORD`. Keep `SYSTEM_AES_KEY` safe:
API keys and other credentials in the database are encrypted with it, and they
cannot be recovered if it is lost.

Build the frontend assets, then build and start the stack:

```bash
./scripts/build_frontend_dist.sh      # runs npm ci + npm run build in frontend/, needed by the frontend image
docker compose up -d --build
docker compose ps                     # wait until the services are healthy
```

The default profile starts the frontend, the Go backend (`app`), `docreader`,
PostgreSQL (ParadeDB), Redis and RustFS, an S3-compatible object store that is
the default file storage (set `STORAGE_TYPE=local` to use a local directory
instead, or point `S3_*` at an external service). Optional profiles add more:
`docker compose --profile docs up -d --build` starts the collaborative
documents service and draw.io (see section K of `.env.example` for the extra
settings it needs), and `--profile full` starts everything optional (Neo4j,
Langfuse, SearXNG, the MCP server, a test OIDC provider).

`.env.example` ships `APK_MIRROR_ARG=mirrors.tencent.com` (an Alpine package
mirror inside mainland China) and `TZ=Asia/Shanghai`. Outside China, empty
the first one and set `TZ` to your own time zone.

### First run

Open the web UI at `http://localhost` (the port is `FRONTEND_PORT`, 80 by
default). There is no default account.

- The first account you register becomes the administrator of the
  deployment. After that, public registration closes. Add people by inviting
  them (workspace settings, members).
- To keep registration open, set `DISABLE_REGISTRATION=false`; to keep it
  closed from the start, set `DISABLE_REGISTRATION=true`. Left unset, it is
  open only until the first account exists.
- Before Q&A works you must configure at least one chat (LLM) model and one
  embedding model: Settings, then Model Management. Ollama on the host and any
  OpenAI-compatible API both work.

| Service | URL |
| --- | --- |
| Web UI | http://localhost (`FRONTEND_PORT`) |
| API | http://localhost:8080 (`APP_PORT`) |
| Swagger UI | http://localhost:8080/swagger/index.html (only when `GIN_MODE` is not `release`) |

By default every published port except the frontend's is bound to localhost.
For anything beyond a single machine, put a reverse proxy with TLS in front of
the frontend, and change the default credentials in `.env` first.

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
| MCP server | [`mcp-server/`](./mcp-server/) | install from source (`pip install ./mcp-server`), not on PyPI — 23 tools |
| Go SDK | [`client/`](./client/) | used by the CLI |
| DeepSeek Harness plugin | [`packages/dsh-yuheng/`](./packages/dsh-yuheng/) | install from source (`dsh plugin --profile web add ./packages/dsh-yuheng`), not on npm |

## Documentation

- [Product documentation](./website-docs/README.md) — getting started, architecture, features, API reference, clients, development (VitePress). Written in Chinese; the quick start and installation pages also have [English versions](./website-docs/en/01-getting-started/02-installation.md)
- [Developer docs](./docs/README.md) — design notes and operations notes, mostly Chinese
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
  production — see the [installation guide](./website-docs/en/01-getting-started/02-installation.md).

## Contributing

Bug reports, fixes and improvements are welcome; read
[`CONTRIBUTING.md`](./CONTRIBUTING.md) first, and note the
[code of conduct](./CODE_OF_CONDUCT.md).

## Attribution

Yuheng is derived in part from the [WeKnora](https://github.com/Tencent/WeKnora)
project, MIT-licensed upstream code is used with notice — see
[`NOTICE`](./NOTICE), [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md), and
[`licenses/upstream-weknora/`](./licenses/upstream-weknora/). <!-- license-check: attribution -->

## License

[MIT](./LICENSE). The license covers the code; the names Yuheng and 玉衡 and the
project's logos are not licensed for use as the name of a modified product — see
[`TRADEMARK.md`](./TRADEMARK.md).
