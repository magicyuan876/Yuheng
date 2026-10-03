# MCP（Model Context Protocol）集成

Yuheng 通过 MCP 对外提供能力：仓库 `mcp-server/` 目录是一个独立的 Python MCP server（包名 **`yuheng-mcp`**，入口命令 `yuheng-mcp-server`），把 Yuheng 的知识库、检索、问答、Wiki 等 REST API 封装成 22 个 MCP 工具，供 Claude Desktop、VS Code Copilot 等外部 MCP 客户端使用。

简单说，这个方向是**让别人用 Yuheng**：在 Claude Desktop、Cursor 等 MCP 客户端里直接查你的知识库、让外部智能体检索与写入知识。方向只有这一个：Yuheng 自己不调用外部 MCP 服务，也不在服务端管理 MCP 服务配置（上游的「设置 → MCP 服务」页面与 `/api/v1/mcp-services` 接口已随内置智能体一起移除）。

此外 `yuheng` CLI 也提供 `yuheng mcp serve`，适合只需要读知识库的本地客户端，见下文「CLI 内置的 MCP server」。需要写入知识库、管理模型或读取 Wiki 时用本文的 `yuheng-mcp`。

`mcp-server/` 是一个独立的 Python 包，包名 **`yuheng-mcp`**（当前 0.1.0，Python ≥ 3.10，依赖 `mcp>=2,<3`、`requests>=2.31.0`、`starlette`、`uvicorn`），核心实现在 `mcp-server/yuheng_mcp_server.py`：`YuhengClient` 用 `requests.Session`（每个线程一个）携带 `X-API-Key` 调 Yuheng REST API，`MCPServer("yuheng-server", version="0.1.0")` 注册工具并通过所选传输对外服务。

::: warning 从源码安装
该包**没有发布到 PyPI**，也没有预构建的镜像，请从源码安装或在本地构建镜像。唯一的启动入口是 `main.py`；安装后同一个入口以命令 `yuheng-mcp-server` 提供。
:::

实现基于 mcp 2.x 的高层 API：每个工具是一个加了 `@mcp.tool()` 装饰器的普通函数，入参 JSON Schema 由类型标注推导，描述取自 docstring，返回的 dict 由框架序列化；扩展工具时新增一个带装饰器的函数即可。`chat` 的阻塞式网络 I/O 被投递到线程池执行，不阻塞 asyncio 事件循环。

## 安装方式

依赖以 `mcp-server/uv.lock` 为准：CI 用它跑测试，第三方许可证清单按它生成，Docker 镜像也只安装它锁定的版本。以下命令与 `pyproject.toml`、`Dockerfile`、`INSTALL.md` 一致：

**源码运行**：

```bash
cd mcp-server
uv sync                   # 按 uv.lock 安装依赖
uv run python main.py
```

**本地开发安装**：

```bash
cd mcp-server
pip install -e .          # 开发模式；或 pip install .
yuheng-mcp-server
```

**Docker**（`mcp-server/Dockerfile`，基于 `python:3.11-slim`，以非特权用户 uid 10001 运行，默认以 Streamable HTTP 传输启动并暴露 8000 端口）。构建分两段：第一段用 `uv export --frozen` 把 `uv.lock` 转成带哈希的 requirements 文件，第二段 `pip install --require-hashes` 只装这些版本，然后从源码运行：

```dockerfile
ENV MCP_HOST=0.0.0.0
ENV MCP_PORT=8000
ENV YUHENG_BASE_URL=http://app:8080/api/v1
EXPOSE 8000
CMD ["python", "main.py", "--transport", "http", "--host", "0.0.0.0", "--port", "8000"]
```

运行容器时必须注入 `MCP_SERVER_AUTH_TOKEN`（HTTP 传输没有它会拒绝启动，见下文「传输方式与网络鉴权」）。

**随 docker compose 启动**：`docker-compose.yml` 里的 `mcp` 服务属于 `full` profile，从 `./mcp-server` 本地构建，容器内 8000 端口映射到宿主机 `${MCP_BIND:-127.0.0.1}:${MCP_PORT:-8082}`，`YUHENG_BASE_URL` 固定为 `http://app:8080/api/v1`，其余变量（`YUHENG_API_KEY`、`MCP_SERVER_AUTH_TOKEN`、`YUHENG_CHAT_TIMEOUT`、`YUHENG_VERIFY_SSL`、`MCP_ALLOWED_UPLOAD_DIRS`）从 `.env` 传入，说明见 `.env.example` 的「H2. MCP Server」一节。

```bash
docker compose --profile full up -d mcp
```

`main.py` 是启动入口：`--check-only` 只做环境检查，`--verbose` 打开调试日志，`--transport/--host/--port` 选择传输。

::: tip stdio 传输下的诊断输出
stdio 传输把 stdout 当作协议通道，任何多余的 `print` 都会污染协议流，客户端会直接判定「启动失败」。因此入口脚本的所有诊断信息一律写 stderr。自行封装启动脚本时务必遵守同样的约定。
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
| `MCP_PORT` | `8000` | 网络传输绑定端口（注意：docker compose 里同名变量表示宿主机映射端口，默认 8082，不会传进容器） |
| `MCP_SERVER_AUTH_TOKEN` | 空 | **SSE/HTTP 传输必填**的共享密钥；未配置时进程直接 `sys.exit(1)` |
| `MCP_ALLOWED_UPLOAD_DIRS` | 空 | 逗号分隔的目录白名单，限制 `create_knowledge_from_file` 可读取的本地路径 |

## 传输方式与网络鉴权

`main()` 支持三种传输（优先级：`--transport` CLI 参数 > `MCP_TRANSPORT` 环境变量 > 默认 stdio）：

| 传输 | 端点 | 适用场景 |
|---|---|---|
| `stdio` | stdin/stdout 管道 | Claude Desktop、VS Code Copilot 等本地客户端（默认） |
| `sse` | `http://host:port/sse`（消息回传地址由 SSE 的 `endpoint` 事件下发，mcp SDK 默认 `/messages/`） | 只支持 SSE 的远程 MCP 客户端 |
| `http` | `http://host:port/mcp` | Streamable HTTP（MCP 2025-03-26 规范），默认以 `stateless_http` 运行 |

SSE 与 HTTP 传输由 `MCPAuthMiddleware`（ASGI 中间件）统一鉴权：客户端必须携带 `Authorization: Bearer <MCP_SERVER_AUTH_TOKEN>` 或 `X-MCP-Auth-Token` header，比较使用 `secrets.compare_digest` 防时序攻击，失败返回 401；`require_network_transport_auth` 确保网络传输在无 token 时根本起不来。

## 暴露的 MCP 工具清单

共 22 个工具，对应 `yuheng_mcp_server.py` 中带 `@mcp.tool()` 装饰器的函数（参数列 `*` 表示 required；`YuhengClient.update_knowledge_base` 方法存在但**未注册**为工具）：

**租户管理**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `create_tenant` | `name`\*, `description`\*, `business`\*, `retriever_engines`, `owner_email` | 创建空间（服务端只对系统管理员、跨空间超管与平台 API Key 开放）。`owner_email` 指定一个已注册用户为 Owner；用平台 Key 调用时必填，用系统管理员的 JWT 调用时留空则管理员自己成为 Owner；未指定检索引擎时默认 postgres 的 keywords + vector 双引擎 |
| `list_tenants` | 无 | 列出所有租户 |

**知识库管理**

| 工具名 | 参数 | 说明 |
|---|---|---|
| `create_knowledge_base` | `name`\*, `description`\*, `embedding_model_id`, `summary_model_id` | 创建知识库；工具固定写入的 chunking：`chunk_size` 1000、`chunk_overlap` 200、分隔符 `["."]`、开启 multimodal |
| `list_knowledge_bases` | 无 | 列出当前租户的知识库 |
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

便利特性：`resolve_kb_id` 会把人类可读的名称（大小写不敏感）解析为 UUID，`hybrid_search`、`chat`（`knowledge_base_ids` 的每一项）与 `create_knowledge_from_text` 同时接受名称与 UUID；其余带 `kb_id` 的工具（`get_knowledge_base`、`list_knowledge`、`create_knowledge_from_file`、`create_knowledge_from_url`、三个 Wiki 工具等）只接受 UUID。名称解析会先查自有知识库再查共享知识库，共享库也能按名字引用；找不到时报错并提示先调用 `list_knowledge_bases` / `list_shared_knowledge_bases`。工具返回 Yuheng API 的 JSON 响应，由框架序列化；REST 调用失败（4xx/5xx）时异常原样抛出，由框架转成工具错误结果返回给客户端。

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

已从源码 `pip install .` 安装时，`command` 可直接写 `yuheng-mcp-server`。

远程部署（Docker / `--transport http`）时，客户端连接 `http://<host>:8000/mcp` 并携带 `Authorization: Bearer <MCP_SERVER_AUTH_TOKEN>`。

## CLI 内置的 MCP server（`yuheng mcp serve`）

`yuheng mcp serve` 把 CLI 变成一个 stdio MCP server（没有 SSE / HTTP 传输），只暴露 8 个工具，刻意不含创建、删除、上传类操作：

| 工具 | 用途 |
| --- | --- |
| `kb_list` / `kb_view` | 列出 / 查看知识库 |
| `doc_list` / `doc_view` / `doc_download` | 列出、查看、下载文档；`doc_download` 单次上限 1 MiB，更大的文档用 `search_chunks` 找相关片段 |
| `chunk_list` | 查看文档分块（检索调试），最多返回 `limit` 条 |
| `search_chunks` | 混合检索（向量 + 关键词） |
| `chat` | RAG 问答，会创建会话与消息记录 |

注册到 MCP 客户端：

```json
{"mcpServers": {"yuheng": {"command": "yuheng", "args": ["mcp", "serve"]}}}
```

认证沿用 CLI 的当前 profile（`--profile` 可指定），或无头环境下的环境变量 `YUHENG_API_KEY`（或 `YUHENG_TOKEN`）+ `YUHENG_HOST`。启动时会先构造客户端：没有可用凭据时进程直接以 `auth.unauthenticated` 退出，而不是握手成功后每次调用都报错。详见 [CLI 文档](../05-clients/02-cli.md)。

## 文件上传路径安全（upload_paths.py）

`create_knowledge_from_file` 读取的是 **MCP server 进程所在机器**的本地文件，`mcp-server/upload_paths.py` 对路径做了防护：

- 拒绝空路径与含 `\x00` 的路径；`os.path.realpath` 规范化后必须是存在的普通文件；
- 白名单目录：`MCP_ALLOWED_UPLOAD_DIRS`（逗号分隔）显式配置时以其为准；未配置时，**网络传输（sse/http）默认只允许当前工作目录**（防远程调用者任意读盘），stdio 传输默认不限制（本地客户端本就拥有该机器权限）；
- `_path_within_root` 用 `os.path.commonpath` 做包含判断，防 `..` 与符号链接逃逸。

在 Docker 镜像里工作目录是 `/app`（即 MCP server 自己的代码目录），所以网络传输下不配置 `MCP_ALLOWED_UPLOAD_DIRS` 时，`create_knowledge_from_file` 只能读 `/app` 下的文件；要导入其他文件，把目录挂进容器（对 uid 10001 可读）并写进 `MCP_ALLOWED_UPLOAD_DIRS`。

## API Key 的权限

MCP server 只是 REST API 的客户端，每个工具能否成功取决于 `YUHENG_API_KEY` 的权限：受限（scoped）的 API Key 只能调用其 capability（`retrieve`、`chat`、`ingest`、`manage_kbs`、`manage_models` 等）与知识库白名单覆盖的接口，超出范围的调用返回错误。例如 `chat` 调用的 `/knowledge-chat/:session_id` 需要 `chat` 能力，`hybrid_search` 需要 `retrieve`。API Key 的能力模型见 [租户、用户与认证授权](01-tenant-auth.md)。

## 鉴权建议

- 给每个外部集成单独建一把**受限 API Key**：只勾选需要的能力（只读检索给 `retrieve`，问答加 `chat`，写入加 `ingest`），并限定可访问的知识库；不要用全量权限的 Key，也不要复用个人账号。
- 定期轮换 Key；网络传输（SSE / HTTP）的 `MCP_SERVER_AUTH_TOKEN` 同样按密钥管理，并保持 `MCP_BIND` 只绑定本机或内网。
- `yuheng mcp serve` 是只读工具面；需要上传或删除时用 CLI 命令或 REST API。CLI 的高风险写操作会以退出码 10（`input.confirmation_required`）要求人工确认后带 `-y` 重试。

## 实现参考

| 路径 | 内容 |
|---|---|
| `mcp-server/yuheng_mcp_server.py` | `YuhengClient`（REST 调用、`resolve_kb_id`、SSE 消费）、22 个 `@mcp.tool()` 工具、三种传输、`MCPAuthMiddleware` |
| `mcp-server/upload_paths.py` | `create_knowledge_from_file` 的路径校验与上传目录白名单 |
| `mcp-server/main.py` | 启动入口 |
| `mcp-server/pyproject.toml`、`uv.lock` | 包定义、`yuheng-mcp-server` 命令与锁定的依赖集 |
| `mcp-server/Dockerfile`、`docker-compose.yml` 的 `mcp` 服务 | 容器化部署 |
| `mcp-server/test_*.py`、`mcp-server/tests/` | 传输、stdout 洁净、文件路径安全等测试 |
| `cli/cmd/mcp/serve.go`、`cli/internal/mcp/tools.go` | `yuheng mcp serve` 与它的 8 个工具 |
| `mcp-server/MCP_CONFIG.md`、`mcp-server/README.md` | 更多客户端的配置示例 |
