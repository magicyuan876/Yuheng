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
| <a href="./README.md"><b>English</b></a> | <b>简体中文</b> |
</p>

# Yuheng —— AI 智能体时代的知识平台

Yuheng 是一个开源的、由大模型驱动的企业级知识平台：它把来自各处的文档与
数据接入进来，解析并组织成可检索的知识，用有引用溯源的问答回答用户问题，
还能把知识库自动生成为可对外发布的 Wiki 站点。

Yuheng 不试图替你当「智能体」。从 0.1.0 开始，它只专注做好一件事：成为
**知识层**。你的 AI 智能体——Claude、Cursor、自研 ReAct 循环，任何东西——
通过 REST 与 MCP 来 Yuheng 获取检索、问答与 Wiki 能力。

## 为什么选择 Yuheng

- **知识进来，答案出去**：上传文档，或接入飞书/Lark、Notion、语雀、RSS、
  GitLab 等数据源；解析、分块、索引全自动，开箱即答，无需编写管道代码。
- **有溯源、可点击的问答**：每条回答都精确链接回来源分块；混合检索 +
  Rerank，可选知识图谱与联网搜索增强，保证答案准确。
- **自动 Wiki**：一条 LLM 流水线把知识库提升为完整 Wiki 站点，支持人工
  修订、版本历史与问题反馈闭环。
- **原生为智能体设计**：Web 界面上人能做的一切，同时都是 REST 接口
  （`/api/v1`，JWT 或细粒度能力域 API Key）和 MCP 工具（23 个）；另附
  Go SDK、CLI 与 DeepSeek Harness 插件。
- **企业级**：多租户工作空间、四级 RBAC、组织与共享空间、审计日志、
  凭据 AES-256-GCM 加密、Langfuse 追踪、限流。

## 核心功能

**📥 接入与解析**
- gRPC 文档解析服务（`docreader`）：PDF、Word、Excel、PPT、HTML、MHTML、
  EPUB、图片、视频等；可选进程内 Rust 解析器（`anydoc`）
- 数据源连接器：飞书/Lark、Notion、语雀、RSS、GitLab、腾讯 ima，凭据
  加密、定时增量同步
- 树形文件夹、多标签、批量操作、分块级编辑与版本历史、自定义元数据

**🔎 检索与问答**
- 可插拔检索引擎：pgvector（默认）、Elasticsearch、OpenSearch、Milvus、
  Weaviate、Qdrant、Doris、腾讯 VectorDB
- 混合检索（向量 + BM25/全文）、Rerank、查询改写与扩展
- FAQ 条目（批量导入、去重）；知识图谱（Neo4j，可选）；内置联网搜索
  （9 家提供商 + 自托管 SearXNG）
- 流式回答：流水线进度时间线 + 可点击引用角标

**📖 自动 Wiki**
- 四阶段 LLM 流水线把知识库变成 Wiki 站点
- 人工编辑、版本历史与回滚；问题报告闭环修复；Wiki 变更并入知识库活动流

**🤖 面向你的 AI 智能体**
- 完整的 `/api/v1` REST API——Swagger UI 位于 `/swagger/index.html`
- 细粒度能力域的 API Key（retrieve、ingest、manage 等）
- [`yuheng-mcp`](./mcp-server/)：23 个 MCP 工具，支持 stdio/SSE/HTTP
- [Go SDK](./client/) 与 [`yuheng` CLI](./cli/)；[DeepSeek Harness 插件](./packages/dsh-yuheng/)

**🏢 平台能力**
- 多租户；工作空间四级角色；组织与共享空间
- 审计日志、Langfuse 可观测性、任务队列看板、限流

## 架构

```
┌─────────────┐   REST / SSE   ┌──────────────────────────────┐
│ Web / CLI   │ ◄────────────► │  Go 后端（Gin，/api/v1）      │
└─────────────┘                │  问答管道 · RAG · Wiki        │
└─────────────┘                │  异步任务（asynq/Redis）       │
┌─────────────┐   MCP (23)     └───────┬──────────────┬───────┘
│ AI 智能体   │ ◄───────────────────── │              │ gRPC（TLS+token）
└─────────────┘                        │              ▼
                               ┌───────┴───────┐  ┌────────────┐
                               │  docreader    │  │ PostgreSQL │
                               │  （Python）   │  │ + pgvector │  可插拔：
                               └───────────────┘  └────────────┘  ES/Milvus/…
```

完整架构见[架构文档](./website-docs/02-architecture/01-overview.md)。

## 快速开始

最快的方式是 Docker Compose：

```bash
git clone https://github.com/magicyuan876/yuheng.git
cd yuheng
cp .env.example .env          # 生产环境请先修改密钥
docker compose pull && docker compose up -d
```

启动后访问：

| 服务 | 地址 |
| --- | --- |
| Web 界面 | http://localhost |
| API | http://localhost:8080 |
| Swagger UI | http://localhost:8080/swagger/index.html |

可选的 Compose profile 可追加 Neo4j、MinIO 与 Langfuse：
`docker compose --profile full up -d`。

其他运行方式：

```bash
make dev-start       # 本地基础设施（Postgres、Redis、docreader、Langfuse）
make dev-app         # 后端（Air 热重载）
make dev-frontend    # Vite 开发服务器
```

## 客户端与集成

| 客户端 | 目录 | 说明 |
| --- | --- | --- |
| Web 界面 | [`frontend/`](./frontend/) | Vue 3 + TDesign |
| CLI | [`cli/`](./cli/) | `yuheng`——可脚本化的 JSON 输出，多 profile |
| MCP 服务 | [`mcp-server/`](./mcp-server/) | `pip install yuheng-mcp`——23 个工具 |
| Go SDK | [`client/`](./client/) | CLI 即基于它 |
| DeepSeek Harness 插件 | [`packages/dsh-yuheng/`](./packages/dsh-yuheng/) | `@magicyuan876/dsh-yuheng` |

## 文档

- [产品文档](./website-docs/README.md)——入门、架构、功能、API 参考、客户端、开发（VitePress）
- [开发者文档](./docs/)——设计说明、常见问题、运维（中英混合）
- [更新日志](./CHANGELOG.md)

## 开发

```bash
make test             # go test -v ./...
make lint             # golangci-lint
cd frontend && npm run type-check && npm test
cd docreader && uv sync && pytest tests/
cd cli && make build && make test
```

## 安全说明

- 所有凭据（API Key、数据源令牌、MCP 密钥）在静态存储时均经 AES-256-GCM 加密。
- 数据源与 URL 导入的外发 HTTP 一律走 SSRF 安全客户端与允许列表。
- 切勿提交 `.env`；部署前请替换 `.env.example` 中的占位密钥。生产环境建议
  内网部署，详见[安装与部署说明](./website-docs/01-getting-started/02-installation.md)。

## 致谢与出处

Yuheng 部分衍生自 [WeKnora](https://github.com/Tencent/WeKnora) 项目，上游
MIT 代码按声明使用——见 [`NOTICE`](./NOTICE)、
[`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md) 与
[`licenses/upstream-weknora/`](./licenses/upstream-weknora/)。

## 许可证

[MIT](./LICENSE)
