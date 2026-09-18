# MCP（Model Context Protocol）集成

Yuheng 通过 MCP 对外提供能力：仓库 `mcp-server/` 目录是一个独立的 Python MCP server（PyPI 包 **`yuheng-mcp`**，入口命令 `yuheng-mcp-server`），把 Yuheng 的知识库、检索、问答、Wiki 等 REST API 封装成 23 个 MCP 工具，供 Claude Desktop、VS Code Copilot 等外部 MCP 客户端使用。

简单说，这个方向是**让别人用 Yuheng**：在 Claude Desktop 里直接查你的知识库、让外部智能体检索与写入知识。

此外 `yuheng` CLI 也提供 `yuheng mcp serve`，把精选的 10 个工具（知识库/文档检索与问答）以 MCP 形式暴露给本地客户端，见 [CLI 文档](../05-clients/02-cli.md)。

`mcp-server/` 是一个独立的 Python 包，PyPI 名 **`yuheng-mcp`**（当前 1.1.1，Python ≥ 3.10，依赖 `mcp>=2,<3`、`requests>=2.31.0`、`starlette`、`uvicorn`），核心实现在 `mcp-server/yuheng_mcp_server.py`：`YuhengClient` 用 `requests.Session` 携带 `X-API-Key` 调 Yuheng REST API，`MCPServer("yuheng-server", version="1.1.1")` 注册工具并通过所选传输对外服务。

::: warning 包名与 API 变更（v1.1.x）
- 本仓库的包名是 `yuheng-mcp`，命令行入口是 `yuheng-mcp-server` / `yuheng-server`。注意该包名尚未在 PyPI 上发布——发布前请先确认名称可用。
- 实现已迁移到 mcp 2.x 的高层 API：工具是加了 `@mcp.tool()` 装饰器的普通函数，入参 JSON Schema 由类型标注自动推导，描述取自 docstring，返回值自动序列化。旧的 `handle_list_tools()` / `handle_call_tool()` 分发写法已移除——扩展工具时只需新增一个带装饰器的函数。
- 阻塞式网络 I/O（`chat`）被投递到线程池执行，不阻塞 asyncio 事件循环。
:::

## 安装方式

以下命令与 `mcp-server/setup.py`、`pyproject.toml`、`Dockerfile`、`INSTALL.md` 一致：

**源码运行**：

```bash
cd mcp-server
pip install -r requirements.txt
python main.py            # 或 python run.py / python run_server.py
```

**从 PyPI 安装**（提供两个 console 入口 `yuheng-mcp-server` 与 `yuheng-server`）：

```bash
pip install yuheng-mcp
yuheng-mcp-server

# 或者不预装，直接用 uvx 运行
uvx --from yuheng-mcp yuheng-mcp-server
```

**本地开发安装**：

```bash
cd mcp-server
pip install -e .          # 开发模式；或 pip install .
yuheng-mcp-server
```

**Docker**（`mcp-server/Dockerfile`，基于 `python:3.11-slim`，默认以 Streamable HTTP 传输启动并暴露 8000 端口）：

```dockerfile
ENV MCP_HOST=0.0.0.0
ENV MCP_PORT=8000
ENV YUHENG_BASE_URL=http://app:8080/api/v1
EXPOSE 8000
CMD ["yuheng-mcp-server", "--transport", "http", "--host", "0.0.0.0", "--port", "8000"]
```

运行容器时必须注入 `MCP_SERVER_AUTH_TOKEN`（HTTP 传输没有它会拒绝启动，见下文「传输方式与网络鉴权」）。

三个入口脚本的分工：`main.py` 是功能最全的主入口（`--check-only` 环境检查、`--verbose`、`--transport/--host/--port`）；`run.py` 是转调 `main.sync_main` 的简化脚本；`run_server.py` 走 `yuheng_mcp_server.run`（stdio 别名）。

::: tip stdio 传输下的诊断输出
stdio 传输把 stdout 当作协议通道，任何多余的 `print` 都会污染协议流，客户端会直接判定「启动失败」。因此入口脚本的所有诊断信息一律写 stderr（#2371）。自行封装启动脚本时务必遵守同样的约定。
:::

## 环境变量

均以 `yuheng_mcp_server.py` / `upload_paths.py` 实际读取为准：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `YUHENG_BASE_URL` | `http://localhost:8080/api/v1` | Yuheng API 基础 URL |
| `YUHENG_API_KEY` | 空 | 租户 API Key，以 `X-API-Key` header 发送 |
| `YUHENG_CHAT_TIMEOUT` | `300` | chat 的 SSE 读超时（秒），非法值回退 300 |
| `YUHENG_VERIFY_SSL` | `true` | 设为 `false` 关闭 SSL 证书校验（仅限自签名证书的开发环境） |
| `MCP_TRANSPORT` | `stdio` | 传输方式：`stdio` / `sse` / `http`（CLI `--transport` 优先） |
| `MCP_HOST` | `127.0.0.1` | 网络传输绑定地址 |
| `MCP_PORT` | `8000` | 网络传输绑定端口 |
| `MCP_SERVER_AUTH_TOKEN` | 空 | **SSE/HTTP 传输必填**的共享密钥；未配置时进程直接 `sys.exit(1)` |
| `MCP_ALLOWED_UPLOAD_DIRS` | 空 | 逗号分隔的目录白名单，限制 `create_knowledge_from_file` 可读取的本地路径 |

## 传输方式与网络鉴权

`main()` 支持三种传输（优先级：`--transport` CLI 参数 > `MCP_TRANSPORT` 环境变量 > 默认 stdio）：

| 传输 | 端点 | 适用场景 |
|---|---|---|
| `stdio` | stdin/stdout 管道 | Claude Desktop、VS Code Copilot 等本地客户端（默认） |
| `sse` | `http://host:port/sse`（消息回传 `/sse/messages/`） | 旧版远程 MCP 客户端 |
| `http` | `http://host:port/mcp` | Streamable HTTP（MCP 2025-03-26 规范），默认以 `stateless_http` 运行 |

SSE 的消息回传路径由 `SSE_MESSAGE_PATH = "/sse/messages/"` 显式指定：迁移到 mcp 2.x 后默认路径与实际挂载点不一致，会让客户端初始化超时。

SSE 与 HTTP 传输由 `MCPAuthMiddleware`（ASGI 中间件）统一鉴权：客户端必须携带 `Authorization: Bearer <MCP_SERVER_AUTH_TOKEN>` 或 `X-MCP-Auth-Token` header，比较使用 `secrets.compare_digest` 防时序攻击，失败返回 401；`require_network_transport_auth` 确保网络传输在无 token 时根本起不来。

## 暴露的 MCP 工具清单

共 23 个工具，对应 `yuheng_mcp_server.py` 中带 `@mcp.tool()` 装饰器的函数（参数列 `*` 表示 required；`YuhengClient.update_knowledge_base` 方法存在但**未注册**为工具）：

**租户管理**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `create_tenant` | `name`\*, `description`\*, `business`\*, `retriever_engines` | 创建租户；未指定检索引擎时默认 postgres 的 keywords + vector 双引擎 |
| `list_tenants` | 无 | 列出所有租户 |

**知识库管理**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `create_knowledge_base` | `name`\*, `description`\*, `embedding_model_id`, `summary_model_id` | 创建知识库；默认 chunking：`chunk_size` 1000、`chunk_overlap` 200、分隔符 `["."]`、开启 multimodal |
| `list_knowledge_bases` | 无 | 列出当前租户自己的知识库 |
| `list_shared_knowledge_bases` | 无 | 列出通过组织/共享空间授权给当前租户的知识库 |
| `get_knowledge_base` | `kb_id`\* | 知识库详情 |
| `delete_knowledge_base` | `kb_id`\* | 删除知识库 |
| `hybrid_search` | `kb_id`\*, `query`\*, `vector_threshold`(0.5), `keyword_threshold`(0.3), `match_count`(5) | 向量 + 关键词混合检索；`kb_id` 支持 UUID **或名称**（`resolve_kb_id` 自动解析） |

**知识管理**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `create_knowledge_from_file` | `kb_id`\*, `file_path`\*, `enable_multimodel`(true) | 从服务器本地文件导入知识；路径经 `upload_paths.resolve_upload_file_path` 校验（见下文「文件上传路径安全」） |
| `create_knowledge_from_url` | `kb_id`\*, `url`\*, `enable_multimodel`(true) | 从网页 URL 导入知识 |
| `create_knowledge_from_text` | `kb_id`\*, `title`\*, `content`\*, `tag_ids`, `status`("publish") | 从原始 Markdown 文本直接建条目（如摘要、粘贴的正文）；`status` 默认 `publish`（立即入库可检索），传 `draft` 则只保存不索引 |
| `list_knowledge` | `kb_id`\*, `page`(1), `page_size`(20) | 分页列出知识条目 |
| `get_knowledge` | `knowledge_id`\* | 知识详情 |
| `delete_knowledge` | `knowledge_id`\* | 删除知识 |

**模型管理**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `create_model` | `name`\*, `type`\*, `description`\*, `source`("local"), `base_url`, `api_key`, `is_default`(false) | 创建模型配置；`type` 为 KnowledgeQA / Embedding / Rerank |
| `list_models` | 无 | 列出所有模型 |
| `get_model` | `model_id`\* | 模型详情 |

**对话**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `chat` | `query`\*, `knowledge_base_ids`, `web_search_enabled`(false) | RAG 流水线（`/knowledge-chat`）：每次调用内部自动创建新会话（返回 `session_id`），检索相关分块后由 LLM 总结，消费 SSE 流并拼装为 `{answer, references}`；强烈建议传 `knowledge_base_ids`（名称或 UUID） |

**分块管理**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `list_chunks` | `knowledge_id`\*, `page`(1), `page_size`(20) | 列出知识条目的文本分块 |
| `delete_chunk` | `knowledge_id`\*, `chunk_id`\* | 删除分块 |

**Wiki（只读）**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `wiki_search` | `kb_id`\*, `query`\*, `limit`(10) | 全文搜索 Wiki 页面（标题、slug、摘要、片段） |
| `wiki_read_page` | `kb_id`\*, `slug`\* | 按 slug 读取整页 Markdown、元数据与出入链 |
| `wiki_index_view` | `kb_id`\*, `limit`(50) | 按类型（entity / concept / summary 等）分组的结构化 Wiki 索引 |

便利特性：`resolve_kb_id` 会把人类可读的名称（大小写不敏感）解析为 UUID，因此 `hybrid_search` / `chat` / `create_knowledge_from_text` 等都同时接受名称与 UUID。名称解析会同时查自有知识库与共享知识库，共享库也能直接按名字引用。所有工具结果统一以格式化 JSON 的 `TextContent` 返回；异常被捕获并返回 `Error executing <name>: ...` 文本。

## 在 Claude Desktop 等客户端中配置

stdio 传输（Claude Desktop 的 `claude_desktop_config.json`）：

```json
{
  "mcpServers": {
    "yuheng": {
      "command": "python",
      "args": ["/path/to/Yuheng/mcp-server/main.py"],
      "env": {
        "YUHENG_BASE_URL": "http://localhost:8080/api/v1",
        "YUHENG_API_KEY": "your-yuheng-api-key"
      }
    }
  }
}
```

已从 PyPI 安装时，`command` 可直接写 `yuheng-mcp-server`，或者用 `uvx` 免安装运行：

```json
{
  "mcpServers": {
    "yuheng": {
      "command": "uvx",
      "args": ["--from", "yuheng-mcp", "yuheng-mcp-server"],
      "env": {
        "YUHENG_BASE_URL": "http://localhost:8080/api/v1",
        "YUHENG_API_KEY": "your-yuheng-api-key"
      }
    }
  }
}
```

远程部署（Docker / `--transport http`）时，客户端连接 `http://<host>:8000/mcp` 并携带 `Authorization: Bearer <MCP_SERVER_AUTH_TOKEN>`。

## 文件上传路径安全（upload_paths.py）

`create_knowledge_from_file` 读取的是 **MCP server 进程所在机器**的本地文件，`mcp-server/upload_paths.py` 对路径做了防护：

- 拒绝空路径与含 `\x00` 的路径；`os.path.realpath` 规范化后必须是存在的普通文件；
- 白名单目录：`MCP_ALLOWED_UPLOAD_DIRS`（逗号分隔）显式配置时以其为准；未配置时，**网络传输（sse/http）默认只允许当前工作目录**（防远程调用者任意读盘），stdio 传输默认不限制（本地客户端本就拥有该机器权限）；
- `_path_within_root` 用 `os.path.commonpath` 做包含判断，防 `..` 与符号链接逃逸。
