---
title: 内置MCP服务管理
tags: [核心功能, MCP, 系统管理, 内置服务]
aliases: [内置MCP, BuiltinMCP, BUILTIN_MCP_SERVICES]
source: BUILTIN_MCP_SERVICES.md
---

# 内置 MCP 服务管理（已移除）

> 服务端「内置 MCP 服务」功能已随 Agent 能力剥离一并移除：系统不再在数据库中内置 MCP 服务配置，原 `mcp_services` 相关表已由迁移 `000089_drop_agent_infra` 删除，对应端点现在返回 `404`。

当前 Yuheng 的角色是供给方——不再消费/托管第三方 MCP 服务，而是通过以下两种方式把知识库能力开放给外部 Agent：

1. **`yuheng-mcp`（Python MCP server）**：官方 PyPI 包，支持 stdio / SSE / HTTP 传输，覆盖知识库 / 文档 / 检索 / RAG 问答 / Wiki 等工具面。配置见 [`mcp-server/MCP_CONFIG.md`](../../../mcp-server/MCP_CONFIG.md)。
2. **`yuheng mcp serve`（CLI 内置 MCP server）**：面向编码 Agent 的受控只读工具面。见 [`cli/README.md`](../../../cli/README.md)。

历史版本（Agent 能力剥离之前）的完整说明保留在 [`docs/BUILTIN_MCP_SERVICES.md`](../../BUILTIN_MCP_SERVICES.md)。
