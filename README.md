<p align="center">
  <img src="./website-docs/public/brand/yuheng-banner.svg" alt="Yuheng 玉衡 — the knowledge layer for AI agents" width="100%">
</p>

<p align="center">
    <a href="https://github.com/magicyuan876/Yuheng/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-1DB592?labelColor=0E1E3C" alt="License"></a>
    <a href="./CHANGELOG.md"><img alt="Version" src="https://img.shields.io/badge/version-0.1.0-1DB592?labelColor=0E1E3C"></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.26-1DB592?labelColor=0E1E3C">
    <img alt="Vue" src="https://img.shields.io/badge/Vue-3.5-1DB592?labelColor=0E1E3C">
    <img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-ParadeDB-1DB592?labelColor=0E1E3C">
</p>

<p align="center">
| <b>English</b> | <a href="./README_CN.md"><b>简体中文</b></a> |
</p>

# Yuheng 玉衡

**The knowledge platform for the age of AI agents.** Yuheng brings in what your
organization knows — files, web pages, Feishu, Notion and more — turns it into
knowledge that is searchable, trustworthy and actually maintained, answers
people's questions with citations, and hands the same capabilities to your AI
agents over REST and MCP.

Yuheng does not try to be your agent. It is the **knowledge layer**: Claude,
Cursor, your own ReAct loop — any agent — comes here to search, ask, read the
Wiki and write knowledge back.

> **The name**: 玉衡 (Yùhéng, Alioth) is the fifth star of the Big Dipper, where
> the bowl meets the handle; 璇玑玉衡 was also the ancient Chinese instrument for
> measuring the heavens. The mark draws the Dipper around it: stars joined into
> one figure, as knowledge is, and the star that weighs and measures, as
> knowledge needs.

## What it does

```
 Ingest             Organize             Answer                Maintain              Expose
 ───────────────    ─────────────────    ──────────────────    ──────────────────    ────────────────
 files and URLs     knowledge bases      hybrid retrieval      duplicates and        REST /api/v1
 data-source sync   chunking, FAQ, tags  cited streaming Q&A   near-copies           MCP (22 tools)
 collaborative docs Auto-Wiki            web search            periodic review       Go SDK · CLI
                    graph (optional)                           answer feedback
                                                               owners and to-dos
```

- **Knowledge in**: upload documents (PDF, Word, Excel, PPT, HTML, EPUB, images,
  audio, video), import web pages, or connect Feishu/Lark wiki and drive, Notion,
  Yuque, RSS, GitLab and Tencent ima with scheduled incremental sync — or write in
  Yuheng's own **collaborative documents**, whose pages flow into the knowledge
  base.
- **Answers out**: vector + BM25 hybrid retrieval, rerank and query rewriting;
  every answer links back to the chunks it came from; optional knowledge graph and
  12 web-search providers (including self-hosted SearXNG).
- **Knowledge that does not quietly rot**: **knowledge health** compares the
  documents of a knowledge base as they change, finding word-for-word copies and
  near-copies that differ (the old policy says 15 days of leave, the new one 10 —
  with the difference highlighted); asks owners to review what nobody has
  confirmed within the review period; and takes answers people marked "not
  helpful" to the documents they cite. Every finding is routed to the person who
  should act, collected in a personal to-do, and settled by superseding one
  document with the other, confirming a document is still right, or dismissing it
  with a reason.
- **Built for agents**: everything the web UI does is a REST endpoint (JWT or
  scoped API keys) and an MCP tool; there is also a Go SDK, the `yuheng` CLI and a
  DeepSeek Harness plugin.
- **Enterprise-ready**: one workspace per company with any number of knowledge
  bases, four member roles and workspace groups, system administrators who
  manage workspaces and accounts, audit logs, AES-256-GCM encrypted
  credentials, login rate limits and lockout, Langfuse tracing.

## A look around

<table>
  <tr>
    <td width="50%" valign="top">
      <img src="./website-docs/public/screenshots/en/health.png" alt="Knowledge health: two leave policies compared, the differences highlighted">
      <p><b>Knowledge health</b>: the 2024 policy grants 15 days of leave, the 2025 one 10. The two near-copies are found on their own, the differences highlighted word by word, and the newer one can supersede the older on the spot.</p>
    </td>
    <td width="50%" valign="top">
      <img src="./website-docs/public/screenshots/en/chat.png" alt="Cited answers">
      <p><b>Cited answers</b>: once the old policy is retired, the same question is answered from the current one, every claim linked to the document it came from.</p>
    </td>
  </tr>
  <tr>
    <td width="50%" valign="top">
      <img src="./website-docs/public/screenshots/en/wiki.png" alt="Auto-Wiki">
      <p><b>Auto-Wiki</b>: as documents arrive, an LLM organizes the knowledge base into interlinked summaries, entities and concepts.</p>
    </td>
    <td width="50%" valign="top">
      <img src="./website-docs/public/screenshots/en/docs.png" alt="Collaborative documents">
      <p><b>Collaborative documents</b>: real-time co-editing with Mermaid, tables and draw.io; in a space bound to a knowledge base, every page becomes knowledge.</p>
    </td>
  </tr>
</table>

<sub>The company, people and policies in the screenshots are made-up demo data.</sub>

## Features

**📥 Ingestion and parsing**
- `docreader` gRPC parsing service: layout analysis, OCR of scans, tables,
  multimodal image captions; optional in-process Rust parser `anydoc`
- Data-source connectors: Feishu/Lark (wiki and drive), Notion, Yuque, RSS,
  GitLab, Tencent ima — encrypted credentials, scheduled incremental sync,
  conflict strategies
- Folder trees, tags, batch operations, chunk-level editing with history, custom
  metadata

**📝 Collaborative documents** (`docs` profile)
- Spaces and page trees, page permissions and restricted pages, locking, trash
- Real-time collaboration (Yjs) or exclusive editing; tables, Mermaid, draw.io,
  attachments, block references, templates
- Comments, mentions, watching and notifications, revision history and restore,
  public share links, import and export
- A space bound to a knowledge base mirrors its pages into it; a restricted page
  never reaches the knowledge base

**🔎 Retrieval and Q&A**
- One retrieval engine, done properly: PostgreSQL + ParadeDB (BM25) + pgvector,
  no separate search cluster to run
- Hybrid retrieval, RRF fusion, rerank, query rewriting and expansion; FAQ
  entries; knowledge graph (Neo4j, optional)
- Streaming answers with a retrieval-progress timeline and clickable citations;
  answers can be rated helpful / not helpful

**📖 Auto-Wiki**
- An LLM pipeline organizes a knowledge base into interlinked Wiki pages, with
  human editing, version history and an issue-feedback loop

**🩺 Knowledge health**
- Content comparison: word-for-word duplicates, and near-copies compared as text
  with the differences highlighted
- Periodic review: a knowledge base's review period sends overdue documents to
  their owners
- Answer feedback: "not helpful" gathers on the cited documents; the question is
  attached only when the person chooses to
- Owners and routing: every document has an owner, every finding goes to the
  person best placed to act; a personal knowledge to-do
- Settling: supersede one of two documents (a docs page is excluded and marked
  superseded, never deleted), confirm still valid, dismiss with a reason

**🤖 For your AI agents**
- The `/api/v1` REST API, Swagger UI at `/swagger/index.html` (non-release mode)
- Scoped API keys (retrieve, chat, ingest, manage_kbs, …), optionally limited to
  knowledge bases
- [`yuheng-mcp`](./mcp-server/): 22 MCP tools over stdio / SSE / HTTP
- [Go SDK](./client/), [`yuheng` CLI](./cli/), [DeepSeek Harness plugin](./packages/dsh-yuheng/)

**🏢 Platform**
- Workspaces with owner / admin / contributor / viewer roles and workspace
  groups; users are global identities placed into workspaces by invitation or
  by a system administrator, who also creates workspaces
- Audit logs and knowledge-base activity, task-queue dashboard, Langfuse, rate
  limiting
- File storage: a local directory or any S3-compatible store, RustFS bundled by
  default

## Architecture

```
┌──────────────────┐  REST / SSE  ┌──────────────────────────────────────────────┐
│ Web UI · CLI     │ ◄──────────► │  Go backend (Gin, /api/v1)                    │
│ Go SDK           │              │  Q&A · retrieval · Wiki · docs · health       │
└──────────────────┘              │  async tasks (asynq / Redis, or in-process)   │
┌──────────────────┐  MCP (23)    └──┬──────────────┬───────────────┬────────────┘
│ AI agents        │ ◄────────────── │              │ gRPC          │ WebSocket
└──────────────────┘                 │              ▼               ▼
          ┌──────────────────────────┴──┐   ┌────────────┐   ┌────────────────┐
          │ PostgreSQL (ParadeDB)       │   │ docreader  │   │ collab (Yjs)   │
          │ data · pgvector · BM25      │   │ (Python)   │   │ docs mode only │
          └─────────────────────────────┘   └────────────┘   └────────────────┘
          Redis · RustFS / S3 · Neo4j (optional) · Langfuse (optional)
```

See the [architecture overview](./website-docs/02-architecture/01-overview.md) (Chinese).

## Project status

Yuheng is in its 0.x preview phase. It began as a fork of Tencent
[WeKnora](https://github.com/Tencent/WeKnora) and now evolves independently
with a substantially reworked architecture: retrieval is unified on PostgreSQL
(ParadeDB), and the product centres on knowledge bases, collaborative documents
and knowledge health; the upstream built-in agent, IM channels and multi-engine
adapters are not carried forward.

- The `/api/v1` REST API may change between 0.x releases; MCP tool names are
  stable.
- The documentation is Chinese-first ([`website-docs/`](./website-docs/)); the
  quick start and installation guide also exist [in English](./website-docs/en/01-getting-started/02-installation.md).
- Docker images are not published yet; build them locally (below).

## Quick start

You need Docker with Compose v2, Node.js and npm (the frontend is built on the
host), and `git`; 4 CPU cores and 8 GB of memory are recommended. The first build
downloads a lot and takes a while.

```bash
git clone https://github.com/magicyuan876/Yuheng.git
cd Yuheng
cp .env.example .env
```

Edit `.env` before the first start: `JWT_SECRET` and `SYSTEM_AES_KEY` are empty
and must be set; the server refuses to start while they are empty, too short, or
one of the old published examples:

```bash
openssl rand -hex 32     # -> JWT_SECRET
openssl rand -hex 16     # -> SYSTEM_AES_KEY (32 hex characters = the 32 bytes AES-256 needs)
```

Change `DB_PASSWORD` and `REDIS_PASSWORD` too. Keep `SYSTEM_AES_KEY` safe: stored
credentials such as API keys are encrypted with it and cannot be recovered
without it.

```bash
./scripts/build_frontend_dist.sh      # builds the frontend assets the frontend image needs
docker compose up -d --build
docker compose ps                     # wait until the services are healthy
```

This starts the frontend, the Go backend (`app`), `docreader`, PostgreSQL
(ParadeDB), Redis and RustFS. Optional profiles: `--profile docs` adds the
collaborative documents service and draw.io (settings in section K of
`.env.example`); `--profile full` starts every optional component (Neo4j,
Langfuse, SearXNG, the MCP server, …). `TZ` defaults to UTC.

### First use

Open `http://localhost` (port `FRONTEND_PORT`, default 80). There is no default
account.

- **The first account to register becomes the administrator of the deployment**;
  registration then closes, and others join by invitation.
  `DISABLE_REGISTRATION=false` keeps it open, `true` closes it from the start.
- Before asking anything, configure at least one chat model and one embedding
  model under Settings → Models; Ollama on the host and any OpenAI-compatible API
  work.
- To have knowledge health ask for reviews, set a review period in the knowledge
  base's basic settings.

| Service | Address |
| --- | --- |
| Web UI | http://localhost (`FRONTEND_PORT`) |
| API | http://localhost:8080 (`APP_PORT`) |
| Readiness | http://localhost:8080/ready |
| Swagger UI | http://localhost:8080/swagger/index.html (when `GIN_MODE` is not `release`) |

By default every published port except the frontend binds to localhost only. To
serve beyond one machine, put a TLS reverse proxy in front of the frontend. Full
steps: [installation](./website-docs/en/01-getting-started/02-installation.md) and
[quick start](./website-docs/en/01-getting-started/03-quickstart.md).

## Clients and integrations

| Client | Directory | Notes |
| --- | --- | --- |
| Web UI | [`frontend/`](./frontend/) | Vue 3.5 + TypeScript + Vite; Tailwind v4 + shadcn-vue (migrating from TDesign screen by screen) |
| MCP server | [`mcp-server/`](./mcp-server/) | Install from source (`pip install ./mcp-server`); 22 tools |
| CLI | [`cli/`](./cli/) | `yuheng`: scriptable JSON output, multiple profiles |
| Go SDK | [`client/`](./client/) | The CLI is built on it |
| DeepSeek Harness plugin | [`packages/dsh-yuheng/`](./packages/dsh-yuheng/) | Install from source |

## Documentation

- [Product documentation](./website-docs/README.md) (Chinese): getting started,
  architecture, features, API reference, clients, development
- Good places to start: [introduction](./website-docs/01-getting-started/01-introduction.md) ·
  [collaborative documents](./website-docs/03-features/07-docs.md) ·
  [knowledge health](./website-docs/03-features/22-knowledge-health.md) ·
  [MCP](./website-docs/03-features/08-mcp.md) ·
  [API overview](./website-docs/04-api/01-api-overview.md) ·
  [extension points](./website-docs/06-development/03-extension-points.md)
- [Changelog](./CHANGELOG.md) · [Roadmap](./docs/ROADMAP.md)

## Development

```bash
make test             # go test ./... (needs Docker: tests run on a real PostgreSQL)
make lint             # golangci-lint
cd frontend && npm run lint && npm test && npm run type-check
cd docreader && uv sync && pytest tests/
cd cli && make build && make test
```

See [`CONTRIBUTING.md`](./CONTRIBUTING.md) for the workflow and conventions, and
follow the [code of conduct](./CODE_OF_CONDUCT.md). Report security issues
privately as described in [`SECURITY.md`](./SECURITY.md).

## Security notes

- Credentials (API keys, data-source tokens, …) are encrypted at rest with
  AES-256-GCM; the server refuses to start with a missing or example key.
- Outbound requests of data sources and URL imports go through SSRF protection
  and an allow-list.
- Login and registration are rate-limited per IP and repeated failures lock the
  account; CORS does not allow credentials by default.
- Never commit `.env`. For production, deploy on an internal network; see the
  [installation guide](./website-docs/en/01-getting-started/02-installation.md).

## Acknowledgements

Yuheng is partly derived from [WeKnora](https://github.com/Tencent/WeKnora) by
Tencent; the upstream MIT code is used as declared in [`NOTICE`](./NOTICE),
[`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md) and
[`licenses/upstream-weknora/`](./licenses/upstream-weknora/). <!-- license-check: attribution -->
The lettering of the logo is Noto Serif CJK (SIL Open Font License 1.1),
converted to outlines.

## License

[MIT](./LICENSE). The license covers the code; the names Yuheng and 玉衡 and the
project's marks are not licensed as the name of a modified product — see
[`TRADEMARK.md`](./TRADEMARK.md).
