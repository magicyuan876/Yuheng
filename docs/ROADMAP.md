# Yuheng Roadmap

Yuheng is a 0.x preview maintained by one person, so this is a list of
directions, not a schedule or a promise. Anything not listed here is not planned.

## Where the project stands

The first release covers: document ingestion and parsing, hybrid retrieval and
cited Q&A, knowledge-graph enrichment, FAQ, Wiki generation, workspaces with
four roles, REST and MCP interfaces, a CLI and a Go SDK. Agents, IM channels,
non-PostgreSQL vector engines and vendor-specific cloud storage providers
were removed from the upstream code on purpose; see the
[changelog](../CHANGELOG.md).

## Direction

- **Stabilize the API.** Settle the `/api/v1` surface and its error format so a
  1.0 can promise compatibility. MCP tool names are already frozen.
- **Retrieval quality.** Semantic and section-aware chunking in addition to the
  rule-based strategies; better evaluation tooling for regression checks.
- **Documents and Wiki.** Show a parsed document's section structure; improve
  the Wiki review flow.
- **Ingestion.** More data-source connectors and more file formats, driven by
  what users ask for.
- **Operations.** Published container images and a documented upgrade path
  between releases (today: build locally, back up before upgrading).
- **Documentation.** More English documentation; the product docs are
  currently Chinese-first.

## Not planned

- A hosted cloud service or a free-quota model service.
- Yuheng as an agent framework. It stays the knowledge layer that agents call
  over REST and MCP.
