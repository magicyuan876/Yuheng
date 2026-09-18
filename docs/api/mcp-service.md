# MCP Service API（已移除）

[返回目录](./README.md)

> 本页文档的端点已全部移除。服务端 MCP 服务管理（`/api/v1/mcp-services`、`/agent/tool-approvals`）随 Agent 能力剥离删除，现在调用返回 `404`；相关数据表由迁移 `000089_drop_agent_infra` 清理。

## 新方式：Yuheng 作为 MCP server 对外服务

当前 Yuheng 的角色是供给方——不再消费/托管第三方 MCP 服务，而是通过以下两种方式把知识库能力开放给外部 Agent：

1. **`yuheng-mcp`（Python MCP server，23 个工具）**：官方 PyPI 包，支持 stdio / SSE / HTTP 传输，覆盖知识库 / 文档 / 检索 / RAG 问答 / Wiki 等工具面。配置见 [`mcp-server/MCP_CONFIG.md`](../../mcp-server/MCP_CONFIG.md)。
2. **`yuheng mcp serve`（CLI 内置 MCP server，8 个只读工具）**：面向编码 Agent 的受控只读工具面（`kb_list` / `kb_view` / `doc_list` / `doc_view` / `doc_download` / `chunk_list` / `search_chunks` / `chat`）。见 [`cli/README.md`](../../cli/README.md)。

更多背景见 [MCP 功能使用说明](../MCP功能使用说明.md) 与 [内置 MCP 服务管理指南](../BUILTIN_MCP_SERVICES.md)。
