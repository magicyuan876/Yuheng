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
| <a href="./README.md"><b>English</b></a> | <b>简体中文</b> |
</p>

# Yuheng —— AI 智能体时代的知识平台

Yuheng 是一个开源的、由大模型驱动的企业级知识平台：它把来自各处的文档与
数据接入进来，解析并组织成可检索的知识，用有引用溯源的问答回答用户问题，
还能把知识库自动生成为可对外发布的 Wiki 站点。

Yuheng 不试图替你当「智能体」。从 0.1.0 开始，它只专注做好一件事：成为
**知识层**。你的 AI 智能体——Claude、Cursor、自研 ReAct 循环，任何东西——
通过 REST 与 MCP 来 Yuheng 获取检索、问答与 Wiki 能力。

## 项目状态

Yuheng 目前是 0.x 预览版，由一位维护者开发。

- `/api/v1` REST 接口在 0.x 各版本之间仍可能变化；MCP 工具名是稳定的，不会改名。
- 文档以中文为主，产品文档（[`website-docs/`](./website-docs/)）是中文优先。快速开始和安装部署两篇另有[英文版](./website-docs/en/01-getting-started/02-installation.md)。
- 首个版本不发布 Docker 镜像，需要在本地构建（见[快速开始](#快速开始)）。

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
- 只用一个检索引擎并把它做扎实：PostgreSQL + ParadeDB（BM25 全文）+ pgvector（向量），
  无需另外部署搜索集群
- 混合检索（向量 + BM25/全文）、Rerank、查询改写与扩展
- FAQ 条目（批量导入、去重）；知识图谱（Neo4j，可选）；内置联网搜索
  （12 家提供商，含自托管 SearXNG）
- 流式回答：流水线进度时间线 + 可点击引用角标

**📖 自动 Wiki**
- 四阶段 LLM 流水线把知识库变成 Wiki 站点
- 人工编辑、版本历史与回滚；问题报告闭环修复；Wiki 变更并入知识库活动流

**🤖 面向你的 AI 智能体**
- 完整的 `/api/v1` REST API——Swagger UI 位于 `/swagger/index.html`
- 细粒度能力域的 API Key（retrieve、ingest、manage 等）
- [`yuheng-mcp`](./mcp-server/)：23 个 MCP 工具，支持 stdio/SSE/HTTP（从源码安装）
- [Go SDK](./client/) 与 [`yuheng` CLI](./cli/)；[DeepSeek Harness 插件](./packages/dsh-yuheng/)

**🏢 平台能力**
- 多租户；工作空间四级角色；组织与共享空间
- 审计日志、Langfuse 可观测性、任务队列看板、限流

## 架构

```
┌─────────────┐   REST / SSE   ┌──────────────────────────────┐
│ Web / CLI   │ ◄────────────► │  Go 后端（Gin，/api/v1）      │
│ Go SDK      │                │  问答管道 · RAG · Wiki        │
└─────────────┘                │  异步任务（asynq/Redis）       │
┌─────────────┐   MCP (23)     └───────┬──────────────┬───────┘
│ AI 智能体   │ ◄───────────────────── │              │ gRPC（TLS+token）
└─────────────┘                        │              ▼
                               ┌───────┴───────┐  ┌────────────┐
                               │  docreader    │  │ PostgreSQL │
                               │  （Python）   │  │ + pgvector │
                               └───────────────┘  └────────────┘
```

完整架构见[架构文档](./website-docs/02-architecture/01-overview.md)。

## 快速开始

本版本不提供预构建镜像，Docker Compose 会从源码构建。需要：带 Compose v2 的
Docker、Node.js 与 npm（前端要先在宿主机上构建）、`git`；建议 4 核 CPU、8 GB
内存，首次构建要下载很多依赖，耗时较长。

```bash
git clone https://github.com/magicyuan876/Yuheng.git
cd Yuheng
cp .env.example .env
```

首次启动前先编辑 `.env`。`JWT_SECRET` 与 `SYSTEM_AES_KEY` 的示例值是公开的，必须
替换；值为空或仍是示例值时，服务会拒绝启动：

```bash
openssl rand -hex 32     # -> JWT_SECRET
openssl rand -hex 16     # -> SYSTEM_AES_KEY（32 个十六进制字符 = AES-256 需要的 32 字节）
```

`DB_PASSWORD` 和 `REDIS_PASSWORD` 也请一并修改。妥善保管 `SYSTEM_AES_KEY`：数据库里
的 API Key 等凭据都用它加密，丢失后无法恢复。

先构建前端静态产物，再构建并启动整套服务：

```bash
./scripts/build_frontend_dist.sh      # 在 frontend/ 里执行 npm ci 与 npm run build，前端镜像依赖它
docker compose up -d --build
docker compose ps                     # 等服务都变成 healthy
```

默认（不带 profile）启动前端、Go 后端（`app`）、`docreader`、PostgreSQL
（ParadeDB）、Redis 和 RustFS。RustFS 是 S3 兼容的对象存储，也是默认的文件存储；
想改用本机目录，设 `STORAGE_TYPE=local`，想用外部存储就配置 `S3_*`。可选 profile：
`docker compose --profile docs up -d --build` 额外启动在线协同文档服务与 draw.io
（还需要的配置见 `.env.example` 的 K 节），`--profile full` 启动全部可选组件
（Neo4j、Langfuse、SearXNG、MCP 服务、测试用 OIDC 等）。

`.env.example` 里的 `APK_MIRROR_ARG=mirrors.tencent.com`（国内 Alpine 镜像源）和
`TZ=Asia/Shanghai` 是面向国内的默认值，在国外部署时请清空前者并把 `TZ` 改成自己的时区。

### 首次使用

浏览器打开 `http://localhost`（端口由 `FRONTEND_PORT` 决定，默认 80）。系统没有默认账号。

- **你注册的第一个账号会成为整个部署的管理员**，之后公开注册自动关闭，其他成员
  通过邀请加入（空间设置里的成员管理）。
- 想一直保持开放注册，设 `DISABLE_REGISTRATION=false`；想从一开始就关闭，设
  `DISABLE_REGISTRATION=true`。不设置时，只在第一个账号出现之前开放注册。
- 问答能用之前，必须先配置至少一个对话（LLM）模型和一个向量（Embedding）模型：
  设置 → 模型管理。宿主机上的 Ollama 和任何 OpenAI 兼容接口都可以。

| 服务 | 地址 |
| --- | --- |
| Web 界面 | http://localhost（`FRONTEND_PORT`） |
| API | http://localhost:8080（`APP_PORT`） |
| Swagger UI | http://localhost:8080/swagger/index.html（仅 `GIN_MODE` 不是 `release` 时可用） |

默认情况下，除前端外，所有发布到宿主机的端口都只绑定在 localhost。要在单机之外使用，
请在前端前面放一个带 TLS 的反向代理，并先改掉 `.env` 里的默认口令。

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
| MCP 服务 | [`mcp-server/`](./mcp-server/) | 从源码安装（`pip install ./mcp-server`），未发布到 PyPI——23 个工具 |
| Go SDK | [`client/`](./client/) | CLI 即基于它 |
| DeepSeek Harness 插件 | [`packages/dsh-yuheng/`](./packages/dsh-yuheng/) | 从源码安装（`dsh plugin --profile web add ./packages/dsh-yuheng`），未发布到 npm |

## 文档

- [产品文档](./website-docs/README.md)——入门、架构、功能、API 参考、客户端、开发（VitePress，中文；快速开始与安装部署另有[英文版](./website-docs/en/01-getting-started/02-installation.md)）
- [开发者文档](./docs/README.md)——设计说明与运维说明，以中文为主
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
  内网部署，详见[安装部署](./website-docs/01-getting-started/02-installation.md)。

## 参与贡献

欢迎提交问题报告、修复和改进，请先阅读 [`CONTRIBUTING.md`](./CONTRIBUTING.md)，
并遵守[行为准则](./CODE_OF_CONDUCT.md)。

## 致谢与出处

Yuheng 部分衍生自 [WeKnora](https://github.com/Tencent/WeKnora) 项目，上游
MIT 代码按声明使用——见 [`NOTICE`](./NOTICE)、
[`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md) 与
[`licenses/upstream-weknora/`](./licenses/upstream-weknora/)。 <!-- license-check: attribution -->

## 许可证

[MIT](./LICENSE)。许可证覆盖的是代码；Yuheng、玉衡这两个名称以及项目标识，
不授权用作修改后产品的名称，详见 [`TRADEMARK.md`](./TRADEMARK.md)。
