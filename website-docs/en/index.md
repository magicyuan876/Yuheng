---
title: Yuheng documentation (English)
titleTemplate: Yuheng
---

# Yuheng documentation

Yuheng is an open-source (MIT) enterprise knowledge platform: the knowledge layer for AI agents. It brings in documents, web pages and content synced from Feishu/Lark, Notion, Yuque, RSS, GitLab and Tencent ima, or written in its own optional collaborative documents; organizes them into knowledge bases, FAQ entries, an LLM-generated Wiki and an optional knowledge graph; answers questions with hybrid retrieval and cited, streamed answers; keeps the knowledge maintained through knowledge health (duplicates, divergent copies, documents due for review, answers people found unhelpful, each routed to a document owner); and exposes all of it through a REST API (`/api/v1`), an MCP server with 23 tools, a Go SDK, the `yuheng` CLI and a DeepSeek Harness plugin.

Yuheng is not an agent framework. It does not run agents; it is what agents call for knowledge they can trust.

**Chinese is the primary language of this documentation.** Only the two pages below are translated into English. Everything else, including the product introduction, the feature guides, the API reference and the architecture notes, is Chinese.

- [Installation](./01-getting-started/02-installation.md): build and run Yuheng with Docker Compose (no Docker images are published, so you build them locally), the one-command deploy script, development mode, Helm and running from source.
- [Quick start](./01-getting-started/03-quickstart.md): register the first account, configure models, upload documents and ask a question, in the UI and with curl.

Yuheng 0.1.0 is a preview, maintained by one developer: the `/api/v1` REST API may change between 0.x releases, and MCP tool names are stable. It began as an independent fork of Tencent's WeKnora and has diverged a long way from it. The source is at [github.com/magicyuan876/Yuheng](https://github.com/magicyuan876/Yuheng); the [README](https://github.com/magicyuan876/Yuheng/blob/main/README.md) is in English.

[Read the documentation in Chinese](../01-getting-started/01-introduction.md)
