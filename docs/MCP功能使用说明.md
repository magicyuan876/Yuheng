# MCP 功能使用说明

> 本文档定位：Yuheng 不**消费** MCP 服务（服务端 MCP 服务管理已移除），而是作为**供给方**——通过 `yuheng-mcp`（Python MCP server）或 `yuheng` CLI 内置的 MCP server，把知识库能力开放给外部 Agent（Claude Desktop、Cursor、DeepSeek Harness、Kimi Code 等）。

## 新定位：Yuheng 是你 Agent 的知识库 MCP Server

MCP（Model Context Protocol）让 AI Agent 以标准协议访问外部工具与数据源。Yuheng 的打开方式是：

- 外部编码 / 问答 Agent 通过 MCP 调用 Yuheng 的检索、文档、Wiki、RAG 问答能力；
- Yuheng 自身不再内置 Agent 运行时，也不再管理第三方 MCP 服务（原「设置 > MCP 服务」页面与 `/api/v1/mcp-services` 端点已删除）。

## 两种接入方式

### 方式一：`yuheng-mcp`（Python MCP server，23 个工具）

Python 包 `yuheng-mcp`（尚未发布到 PyPI，从 `mcp-server/` 源码安装），支持 stdio / SSE / HTTP 三种传输，工具覆盖：租户与知识库管理、文档导入（文件 / URL / 文本）、混合检索、RAG 问答（`chat`，每次调用自动创建会话）、分块管理、Wiki 搜索与阅读、模型管理。

配置步骤（Claude Desktop 示例）：

```json
{
  "mcpServers": {
    "yuheng": {
      "command": "uvx",
      "args": ["--from", "yuheng-mcp", "yuheng-mcp-server"],
      "env": {
        "YUHENG_API_KEY": "your_api_key_here",
        "YUHENG_BASE_URL": "http://localhost:8080/api/v1"
      }
    }
  }
}
```

更详细的安装与配置（uv / pip / Docker、SSE / HTTP 部署）见 [`mcp-server/MCP_CONFIG.md`](../mcp-server/MCP_CONFIG.md)。

### 方式二：`yuheng` CLI 内置 MCP server（8 个只读工具）

`yuheng mcp serve` 把 CLI 变成一个 stdio MCP server，对外只暴露经过甄选的只读工具面：

| 工具 | 用途 |
| --- | --- |
| `kb_list` / `kb_view` | 列出 / 查看知识库 |
| `doc_list` / `doc_view` / `doc_download` | 列出、查看、下载文档（1 MiB 上限） |
| `chunk_list` | 查看文档分块（RAG 检索调试） |
| `search_chunks` | 混合检索（向量 + 关键词） |
| `chat` | 流式 RAG 问答（无会话时自动创建） |

注册方式：

```json
{"mcpServers": {"yuheng": {"command": "yuheng", "args": ["mcp", "serve"]}}}
```

认证复用 CLI 的 profile / 环境变量（`YUHENG_API_KEY` + `YUHENG_HOST`），详见 [`cli/README.md`](../cli/README.md) 与 [`cli/AGENTS.md`](../cli/AGENTS.md)。

## 鉴权建议

- 为 Agent 集成创建**最小权限的权限范围 API Key**（能力级授权 + 按 KB 限制），不要复用人账号密码；
- 生产环境定期轮换 Key；Yuheng 对凭据做 AES-256-GCM 静态加密；
- CLI MCP 面为只读设计：变更操作（上传、删除）请走 CLI 命令或 REST API，并遵守 exit-10 人工确认协议。

## 变更备忘（历史参考）

以下功能已随 Agent 能力剥离移除，此处保留作历史参考：

| 原功能 | 现状 | 替代方案 |
| --- | --- | --- |
| 服务端 MCP 服务管理（`设置 > MCP 服务`、`/api/v1/mcp-services`） | 已移除 | 由 Agent 宿主自行管理 MCP server；Yuheng 侧使用 `yuheng-mcp` / `yuheng mcp serve` |
| 内置 MCP 服务（数据库内置、全空间可见） | 已移除 | 同上 |
| Agent 会话内调用 MCP 工具（`@MCP` 提及、OAuth 远程服务） | 已移除（ReAct 引擎移除） | 由外部 Agent 通过 MCP 调用 Yuheng |
