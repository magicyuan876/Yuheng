# 可观测性与审计

线上跑起来之后，你会关心三类问题：某次回答为什么慢、为什么答错；谁在什么时候改了什么；后台任务有没有堆积。Yuheng 分别提供了追踪、审计日志和队列面板来回答它们。

| 想知道什么 | 去哪看 |
| --- | --- |
| 某次问答检索了什么、调了几次模型、花了多少 token | 接入 Langfuse 后在 Langfuse 里看完整调用链 |
| 谁改了知识库 / 成员 / 系统设置 | 知识库设置的「活动记录」；空间级事件在「设置 → 成员管理」页的审计日志；平台级事件在「设置 → 系统管理 → 审计日志」（系统管理员） |
| 后台解析、摘要、Wiki 任务是否堆积或失败 | 「设置 → 系统管理 → 任务队列」（系统管理员） |
| 服务是否存活 / 是否可以接流量 | `GET /health`（存活）、`GET /ready`（就绪） |
| 一次请求在各服务的日志里怎么串起来 | 按响应头里的 `X-Request-ID` 检索日志 |

Langfuse 可以接 Langfuse Cloud，也可以用 compose 的 `langfuse` profile 在本地自建，步骤见 [3.1 接入 Langfuse](#langfuse-setup)；日志相关的环境变量见 [2.1](#logging-env)。

下面按日志、追踪、审计、限流、健康检查逐项展开。

## 1. 可观测性数据流总览

```mermaid
flowchart TB
    subgraph HTTP["HTTP 请求路径 (Gin)"]
        RID["middleware.RequestID<br/>(X-Request-ID 生成/透传)"]
        RLOG["middleware.Logger<br/>(请求/响应体脱敏采集)"]
        LFMW["langfuse.GinMiddleware<br/>(白名单路径开 Trace)"]
        RBAC["middleware RBAC<br/>(拒绝时 LogDenied)"]
        H["业务 Handler"]
        RID --> RLOG --> LFMW --> RBAC --> H
    end

    subgraph ASYNC["异步任务路径 (asynq worker)"]
        INJ["InjectTracing<br/>(traceparent 写入 payload)"]
        AMW["langfuse.AsynqMiddleware<br/>(续接 trace + SPAN)"]
        WH["任务 Handler"]
        INJ --> AMW --> WH
    end
    H -->|"Enqueue(payload 内嵌 TracingContext)"| INJ

    subgraph SINKS["数据汇聚"]
        STDOUT["stdout + LOG_PATH 文件<br/>(lumberjack 轮转: 50MB x 3, 28 天, gzip)"]
        LLMDBG["llm_debug/ 按 request_id 分文件<br/>(LLM_DEBUG_LOG, 7 天清理)"]
        LFB["Langfuse / LiteFuse 后端<br/>POST /api/public/otel/v1/traces<br/>(OTLP HTTP + Basic Auth)"]
        ADB["audit_logs 表 (append-only)"]
        DLDB["task_dead_letters 表"]
    end

    RLOG --> STDOUT
    H --> STDOUT
    WH --> STDOUT
    H -.->|"LLMDebugLog"| LLMDBG
    WH -.->|"LLMDebugLog"| LLMDBG
    LFMW -->|"BatchSpanProcessor 批量导出"| LFB
    AMW --> LFB
    GEN["模型 langfuse_wrapper<br/>(chat / embedding / rerank / vlm / asr)"] --> LFB
    H --> GEN
    WH --> GEN
    RBAC -->|"rbac.access_denied (1 分钟去重)"| ADB
    H -->|"AuditLogService.Log"| ADB
    WH -->|"重试耗尽"| DLDB

    subgraph READERS["查询面"]
        API1["GET /tenants/:id/audit-log"]
        API2["GET /knowledge-bases/:id/activity"]
        API3["GET /system/admin/audit-log"]
        RET["AuditLogRetentionRunner<br/>(每日清扫, 默认保留 90 天)"]
    end
    ADB --> API1
    ADB --> API2
    ADB --> API3
    RET -->|"DeleteOlderThan"| ADB
```

## 2. 日志系统（`internal/logger`）

### 2.1 环境变量 {#logging-env}

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LOG_LEVEL` | `debug` | `debug` / `info` / `warn`（或 `warning`）/ `error` / `fatal`，无效值回退 `debug`；生产建议 `info` |
| `LOG_PATH` | 空（只写 stdout） | 落盘路径；设置后同时写 stdout 与该文件，按 2.3 的规则轮转 |
| `LOG_FORMAT` | 空（内置默认格式） | 自定义单行模板，占位符见下 |
| `LLM_DEBUG_LOG` | 关闭 | 完整记录每次模型调用，见 2.4 |

`LOG_LEVEL` / `LOG_PATH` / `LOG_FORMAT` 在 `main` 加载 `.env` 之后由 `logger.ConfigureFromEnv()` 重新生效，改完重启进程即可。

默认格式形如：

```
INFO [2026-05-21 10:20:30.123] [req-abc k1=v1] file.go:42[fn] | message body
```

`LOG_FORMAT` 的占位符：

| 占位符 | 含义 |
| --- | --- |
| `%d` | 时间戳（`2006-01-02 15:04:05.000`） |
| `%level` | 级别（`DEBUG` / `INFO` / `WARNING` / `ERROR` / `FATAL`）；开启颜色时只有这一段着色 |
| `%thread` | goroutine ID；模板不引用它时不会调用 `runtime.Stack`，没有额外开销 |
| `%logger` | caller（`file.go:line[func]`），超过 50 个字符时保留末尾 50 个 |
| `%traceId` | 请求 ID（context 里的 `request_id`） |
| `%msg` | 消息正文 + 其余结构化字段（`key=value`，按 key 升序） |

```bash
export LOG_FORMAT='[%d] %level %thread %logger %traceId | %msg'
# 输出：[2026-05-21 10:20:30.123] INFO 17 service.go:88[Handle] req-abc | hello extra=ok
```

模板模式下结构化字段只能跟在 `%msg` 后面，不能单独抽成占位符；需要按字段检索时保持默认格式。

### 2.2 格式与级别

- 底层为**私有** logrus 实例（`appLogger`，避免外部依赖改写全局 logrus 导致日志丢失），自定义 `CustomFormatter`。
- 默认单行格式：`LEVEL[时间戳] [request_id 字段...] caller | message`，caller 为 `文件:行[函数名]`（`addCaller`）。
- 可通过 `LOG_FORMAT` 环境变量提供模板，占位符：`%d`=时间、`%level`=级别、`%thread`=goroutine ID（仅模板引用时才取，避免每条日志跑 `runtime.Stack`）、`%logger`=caller、`%traceId`=request_id、`%msg`=消息+结构化字段。单趟 `strings.NewReplacer` 替换避免二次替换问题。
- 级别由 `LOG_LEVEL` 控制（`debug`/`info`/`warn`/`error`/`fatal`，未设置或非法时**默认 debug**）。
- 颜色：stdout 是终端时启用 ANSI 颜色；非终端（Docker 采集）禁用；写文件时 `ansiStripWriter` 剥离 ANSI 序列保持纯文本。
- 结构化字段 API：`logger.WithField(ctx, k, v)` / `WithFields` 把带字段的 entry 存进 context（`types.LoggerContextKey`），后续 `logger.Infof(ctx, ...)` 自动携带；`WarnWithFields` 专用于审计相关事件（跨租户探测、不变量破坏），便于日志聚合器按 tenant/资源索引。
- `CloneContext` 在派生后台 goroutine 时复制关键 context 键（tenant/user/request_id/角色/语言等），并同时保留 Langfuse `*Trace` 句柄与**活跃的 OTel span**，防止子 span 变成孤儿 trace。

### 2.3 输出与轮转

`ConfigureFromEnv()`（init 时执行，`main` 加载 `.env` 后可重调）：始终写 stdout；`LOG_PATH` 非空时通过 lumberjack 附加落盘：

```go
// internal/logger/logger.go openLogFile()
return &lumberjack.Logger{
    Filename:   logPath,
    MaxSize:    50, // megabytes
    MaxBackups: 3,
    MaxAge:     28, // days
    Compress:   true,
}, nil
```

### 2.4 LLM 调试日志（`internal/logger/llm_logger.go`）

`LLM_DEBUG_LOG=true|1|<目录>` 开启后，每次模型调用（Chat / Chat Stream / Embedding / Rerank / VLM）都会把**完整**的输入消息、工具调用、输出与错误写到 `llm_debug/` 目录，**同一 request_id 的所有调用追加到同一个文件**（`<request_id>.log`），便于还原一次会话内的全部模型交互。目录中超过 7 天的文件在启动时后台清理（`cleanupOldDebugFiles`）。

### 2.5 请求日志中间件（`internal/middleware/logger.go`）

- `RequestID()`：读取或生成 `X-Request-ID`，写回响应头，并把 request_id 与带字段的 logger 一起放入 gin context 与 `http.Request` context —— 全链路日志（含 asynq worker 侧透传的 session 标签）都能按 request_id 关联。
- `Logger()`：记录 method、path（query 经 `sanitizeQuery` 抹掉 `token`/`code`/`state` 等 OAuth 敏感参数）、status_code、latency、client_ip、size，以及最多 10KB 的请求/响应体。请求/响应体经 `sensitiveFieldRegex` 脱敏（password/token/api_key/secret/private_key 等字段值替换为 `"***"`，兼容 snake_case/camelCase）；SSE 响应体记为 `[SSE流式响应，已跳过]`；`/assets/` 与 wiki stats 轮询路径直接跳过。
- 信任代理：`r.SetTrustedProxies(...)`（`YUHENG_TRUSTED_PROXIES`）防止伪造 `X-Forwarded-For` 绕过基于 `ClientIP` 的限流。

## 3. Langfuse 追踪（`internal/tracing/langfuse`）

Yuheng 的分布式追踪不是通用 OTel 接入，而是**基于 OpenTelemetry Go SDK 实现的 Langfuse v3+ / LiteFuse 客户端**：span 携带 Langfuse 语义约定属性（`langfuse.observation.*`，镜像 langfuse-python v4 的 `_client/attributes.py`），经 OTLP/HTTP 导出到 `POST <host>/api/public/otel/v1/traces`。完全 opt-in：未启用时所有入口都是零成本 no-op。

### 3.1 接入 Langfuse {#langfuse-setup}

只要同时设置了 `LANGFUSE_PUBLIC_KEY` 与 `LANGFUSE_SECRET_KEY` 就会启用（也可用 `LANGFUSE_ENABLED` 显式开关），重启 app 后启动日志出现：

```
[Langfuse] enabled host=https://cloud.langfuse.com flush_at=15 flush_interval=3s sample_rate=1.00 (OTLP/OTel SDK)
```

**接 Langfuse Cloud**：在 cloud.langfuse.com 的 `Project Settings → API Keys` 生成一对 key，写进 `.env`（`LANGFUSE_HOST` 默认就是 `https://cloud.langfuse.com`，美区填 `https://us.cloud.langfuse.com`），然后 `docker compose up -d app`。

**自建（`langfuse` profile）**：

```bash
docker compose --profile langfuse up -d   # 首次启动 ClickHouse 迁移约 1-2 分钟
# 浏览器打开 http://localhost:3000 注册管理员，在 Project Settings → API Keys 生成 key
# .env 里填 LANGFUSE_PUBLIC_KEY / LANGFUSE_SECRET_KEY，并设 LANGFUSE_HOST=http://langfuse-web:3000
docker compose up -d app
```

这套栈复用 Yuheng 已有的 PostgreSQL（一次性的 `langfuse-db-init` 容器在同一实例里建独立的 `langfuse` 库，库名 `LANGFUSE_DB_NAME`）和 Redis（独立 DB 号 `LANGFUSE_REDIS_DB`，默认 1，Yuheng 用 0），新增的常驻容器是 `langfuse-web`、`langfuse-worker`、`langfuse-clickhouse` 与专用的 `langfuse-minio`。Web 端口 `LANGFUSE_WEB_PORT`（默认 3000），MinIO 端口 9100 / 9101，均默认只绑定本机。填了 `LANGFUSE_INIT_*` 时首次启动直接创建组织、项目、管理员与项目 key，跳过界面注册。

生产环境务必替换开发用的默认值：用 `openssl rand -base64 32` 生成 `LANGFUSE_SALT` 与 `LANGFUSE_NEXTAUTH_SECRET`，`openssl rand -hex 32` 生成 `LANGFUSE_ENCRYPTION_KEY`，并修改 `LANGFUSE_CLICKHOUSE_PASSWORD` 与 `LANGFUSE_MINIO_PASSWORD`。Langfuse 建议 Redis 使用 `maxmemory-policy noeviction`，避免内存紧张时丢队列任务。完整变量见 `.env.example` 的 Langfuse 两节。

**本地开发**：`docker-compose.dev.yml` 也有对称的 `langfuse` profile；app 用 `go run ./cmd/server` 跑在宿主机时，`LANGFUSE_HOST` 要填 `http://localhost:3000` 而不是容器名。

**Helm**：在 `app.extraEnv` 里注入三个变量，Secret Key 放 Kubernetes Secret（`valueFrom.secretKeyRef`），不要写进 values.yaml。

**验证**：发起一次知识问答（`POST /api/v1/knowledge-chat/:session_id`）或检索（`POST /api/v1/knowledge-search`），约 3 秒后（或攒够 `LANGFUSE_FLUSH_AT` 条）Langfuse 的 Traces 页出现对应 trace。

**禁用**：删掉或留空两个 key，或设 `LANGFUSE_ENABLED=false`，重启即可，所有埋点退回 no-op。

### 3.2 配置（环境变量，`config.go`）

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LANGFUSE_ENABLED` | 有公私钥时自动启用 | 总开关，接受 `true/false/1/0/yes/no` |
| `LANGFUSE_HOST` | `https://cloud.langfuse.com` | Langfuse/LiteFuse 基址（可自建） |
| `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_SECRET_KEY` | — | Basic Auth 项目凭证 |
| `LANGFUSE_RELEASE` / `LANGFUSE_ENVIRONMENT` | — | 附加到每条 trace 用于 UI 过滤 |
| `LANGFUSE_FLUSH_AT` | 15 | 批量导出批大小（BatchSpanProcessor `MaxExportBatchSize`） |
| `LANGFUSE_FLUSH_INTERVAL` | 3s | 批量导出最大间隔（`BatchTimeout`）；Go duration 写法，纯数字按秒 |
| `LANGFUSE_QUEUE_SIZE` | 2048 | 内存缓冲上限；满了之后新 span 被丢弃，不拖慢业务 |
| `LANGFUSE_REQUEST_TIMEOUT` | 10s | 单次 ingestion HTTP 超时 |
| `LANGFUSE_SAMPLE_RATE` | 1.0 | `ParentBased(TraceIDRatioBased)` 采样率，0..1；`0` 视为 1.0 |
| `LANGFUSE_DEBUG` | false | 批量发送错误的详细日志 |

### 3.3 导出器（`exporter.go`）

OTLP/HTTP exporter，`Authorization: Basic base64(public:secret)`；`x-langfuse-ingestion-version: 4` 是 Langfuse v3/LiteFuse OTel 直写路径的必需门槛头（缺失会返回 400），`x-langfuse-sdk-name/version` 为兼容标记。`Manager`（`manager.go`）持有独立的 `TracerProvider`（`service.name=yuheng` resource），刻意**不**调用 `otel.SetTextMapPropagator` 等全局 OTel 变更，避免影响进程内其他 OTel 埋点；W3C `TraceContext` propagator 为包级私有值。

### 3.4 观测模型与埋点点位

三种句柄（`tracer.go`）：`Trace`（根，一次请求）、`Span`（非 LLM 的逻辑工作单元）、`Generation`（一次模型调用，含 `TokenUsage` token 统计与流式 time-to-first-token `MarkCompletionStart`）。父子关系通过 OTel span context 自动建立；无 trace 时自动开 auto-trace 防止孤儿 span。

主要埋点：

| 点位 | 源码 | 产出 |
| --- | --- | --- |
| HTTP 入口 | `middleware.go` `GinMiddleware` | 对 `shouldTrace` 白名单路径（knowledge-chat / knowledge-search / 各类 ingestion POST/PUT / FAQ 导入 / wiki auto-fix / evaluation / initialization 检测等）开根 Trace，名称为 `METHOD /path`，metadata 含 http.method/path/query/request_id，输出为 status 与 response.size；提取上游 W3C `traceparent` 头继承外部调用方 trace id |
| asynq worker | `asynq.go` `AsynqMiddleware` | 从 payload 恢复 traceparent 续接 HTTP trace，否则新开 `asynq.<task_type>` trace；包一层 SPAN，metadata 含 task_id/queue/retry/max_retry/payload_bytes；payload 只预览前 1KB |
| 入队侧注入 | `asynq.go` `InjectTracing` + `internal/types/tracing.go` `TracingContext` | 把 traceparent、user/session 标签以 `lf_*` JSON 字段嵌入任务 payload，跨进程传递 |
| 模型调用 | `internal/models/{chat,embedding,rerank,vlm,asr}/langfuse_wrapper.go` | 每次调用一个 Generation（模型名、输入、参数、输出、token usage、错误） |
| 检索/重排摘要 | `retrieval_obs.go` | `SummarizeRetrieveOutput` / `SummarizeSearchResults` 等把召回结果压缩成 top-25 预览（rank/chunk_id/score/160 字符 preview），避免全文进 trace |
| 管线阶段 | `chat_pipeline/progress.go` 等 | 检索/重排/合并等阶段的 span 与 pipeline 日志，经 `logger.CloneContext` 保持与 HTTP 根 trace 同树 |

上报内容（span 属性，`events.go`）：`langfuse.observation.type/input/output/metadata/model.name/model.parameters/usage_details/completion_start_time`、`langfuse.trace.name/input/output/metadata/tags`、`user.id`（显式 user 或 `tenant:<id>`）、`session.id`、`langfuse.environment/release`。

```mermaid
flowchart LR
    A["GinMiddleware<br/>Trace: POST /api/v1/knowledge-chat"] --> B["Span: pipeline stages (检索/重排/合并)"]
    B --> C["Generation: chat (LLM 改写/回答)"]
    B --> D["Generation: embedding (检索)"]
    B --> E["Generation: rerank"]
    A --> F["InjectTracing -> asynq payload"]
    F --> G["AsynqMiddleware<br/>Span: asynq.document:process"]
    G --> H["Generation: embedding / vlm / chat"]
```

### 3.5 token 用量口径

| 模型类型 | Generation 名称 | 计量 |
| --- | --- | --- |
| Chat | `chat.completion` / `chat.completion.stream` | 用模型返回的 prompt / completion / total tokens；缓存命中与写入 token（`cached_tokens`、DeepSeek hit tokens、Anthropic cache read/creation）归一化后单独上报；流式记录首 token 时间 |
| Embedding | `embedding.embed` / `embedding.batch_embed` | 模型没返回 usage 时按每条 `字符数/4 + 1` 估算 |
| Rerank | `rerank` | 按查询与全部文档的 `字符数/4 + 1` 估算 |
| VLM | `vlm.predict` | 不上传图片字节，只记图片数量与大小 |
| ASR | `asr.transcribe` | 以秒（`SECONDS`）计量，取转写结果的音频时长 |

`userId` 取登录用户，没有时为 `tenant:<id>`；`sessionId` 取 URL 里的会话 ID。要在 Langfuse 里核算费用，可以在它的 `Settings → Models` 里给自定义模型（本地 Ollama 等）配置单价。

### 3.6 调优与排查

- 高流量时调大 `LANGFUSE_FLUSH_AT`（50–100）和 `LANGFUSE_QUEUE_SIZE`（如 8192），或把 `LANGFUSE_SAMPLE_RATE` 降到 0.1 只采样一部分请求；Langfuse 与 Yuheng 部署得越近，上报延迟越低。
- 启动日志没有 `[Langfuse] enabled`：进程没读到两个 key，容器里 `env | grep LANGFUSE` 核对。
- 控制台看不到 trace：临时打开 `LANGFUSE_DEBUG=true` 看上报错误，常见原因是 `LANGFUSE_HOST` 写错、防火墙拦截、key 轮换后没更新。
- 部分数据缺失：调大 `LANGFUSE_QUEUE_SIZE`，并确认 Langfuse ingest 没有返回 429 / 503。
- token 数为 0：该模型的响应里没有 usage（部分本地 / 自建模型），在模型侧开启 usage 统计。

## 4. 审计日志

### 4.1 数据模型（`internal/types/audit_log.go`）

`audit_logs` 表 **append-only**（无 UpdatedAt、无软删除），单调 id 同时作为主键与游标：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | uint64 自增 | 主键 + 分页游标（`WHERE id < after_id ORDER BY id DESC`） |
| `tenant_id` | uint64 | 空间；`0` = 系统级（system-scope）事件 |
| `actor_user_id` / `actor_role` | varchar | 操作者与其当时角色（系统触发时为空） |
| `action` | varchar(64) | 点分命名 `<area>.<event>`（见 4.2） |
| `scope_type` / `scope_id` | varchar | 资源作用域（如 `knowledge_base` + kbID，驱动 KB 活动页） |
| `target_type` / `target_id` / `target_user_id` | varchar | 具体目标资源 / 用户 |
| `request_path` / `request_method` | varchar | 路由模板（非原始 URL，防游标爆表；原始 URL 存 Details.raw_path） |
| `outcome` | varchar(16) | `success` / `accepted`（异步已受理未终态）/ `denied` / `failed` / `partial` / `canceled` |
| `details` | jsonb | 动作特定负载；密钥值**绝不**入库（如 vector_store 只记变更字段名） |
| `created_at` | timestamp | 保留策略清扫依据 |

### 4.2 审计动作清单

| 分组 | 动作 |
| --- | --- |
| RBAC / 成员 | `rbac.member_added`、`rbac.member_removed`、`rbac.member_role_changed`、`rbac.member_left`、`rbac.access_denied`、`rbac.invitation_sent`、`rbac.invitation_accepted`、`rbac.invitation_declined`、`rbac.invitation_revoked`、`rbac.invitation_expired` |
| 向量库 | `vector_store.created`、`vector_store.updated`、`vector_store.deleted` |
| 系统管理（tenant_id=0） | `system.setting_changed`、`system.admin_promoted`、`system.admin_revoked`、`system.user_password_reset`、`system.user_created`、`system.api_key_created`、`system.api_key_revoked` |
| 运行时队列操作（tenant_id=0） | `system.queue_task_retried`、`system.queue_task_deleted`、`system.queue_task_run_now`、`system.queue_task_cancelled`、`system.queue_archived_purged` |
| 知识库 | `kb.created`、`kb.updated`、`kb.deleted`、`kb.duplicated`、`kb.clone_started`、`kb.clone_completed`、`kb.clone_failed`、`kb.share_added`、`kb.share_permission_changed`、`kb.share_removed` |
| 知识 | `knowledge.created`、`knowledge.updated`、`knowledge.deleted`、`knowledge.batch_deleted`、`knowledge.reparse_started`、`knowledge.parse_canceled`、`knowledge.move_started`、`knowledge.move_completed`、`knowledge.move_failed`、`knowledge.owner_changed`、`knowledge.reviewed` |
| 标签 / 数据源 | `tag.created`、`tag.updated`、`tag.deleted`、`datasource.created`、`datasource.updated`、`datasource.deleted`、`datasource.sync_started`、`datasource.sync_completed`、`datasource.sync_failed`、`datasource.paused`、`datasource.resumed` |
| Wiki / FAQ | `wiki.content_changed`、`faq.import_started`、`faq.import_completed`、`faq.import_failed` |
| 知识健康 | `finding.status_changed`、`finding.scan_requested`、`finding.assigned`、`finding.superseded`（见[知识健康](22-knowledge-health.md)） |

### 4.3 写入路径（service + middleware）

- `auditLogService.Log`（`internal/application/service/audit_log.go`）是规范写入口：默认 `outcome=success`、填充 `CreatedAt`；**写失败只记 ERROR 日志不向上传播** —— 审计失败绝不能中断业务操作。
- `LogDenied` 记录 RBAC 中间件拒绝：以 `(tenant_id, actor, action=rbac.access_denied, route 模板)` 为键做 **1 分钟滑动窗口去重**（`denyDedupWindow`，`repo.CountSinceForDedup`），防止探测客户端灌满表（100 RPS 打同一端点每分钟只产生 1 行）；用路由模板而非原始 URL 作为 dedup 键，防止遍历 UUID 绕过窗口。stderr 侧的 `[rbac] role insufficient` 日志不受去重影响，每次拒绝都打。
- `middleware/audit_provider.go` 的 `AuditServiceProvider` 把 service 注入 gin context（键 `yuheng.audit_service`），RBAC 中间件经 `AuditServiceFromContext` 取用，nil 安全（无 Redis 模式可不配审计）。

### 4.4 查询 API（`internal/handler/audit_log.go`）

| 路由 | 权限 | 说明 |
| --- | --- | --- |
| `GET /api/v1/tenants/:id/audit-log` | PathTenantMatch + Admin | 空间审计流；只返回 `scope_type=''` 的空间级行（`UnscopedOnly`） |
| `GET /api/v1/knowledge-bases/:id/activity` | KB 创建者或空间 Admin，且必须是 owner 空间（组织共享消费方不可读） | `scope_type=knowledge_base` + `scope_id=kbID` 的 KB 活动投影 |
| `GET /api/v1/system/admin/audit-log` | SystemAdmin（或带 `system_audit_read` 能力的平台 API Key） | `tenant_id=0` 的平台级事件（settings / promote / queue 操作等） |

统一查询参数：`after_id`（游标，返回 id 更小的行）、`limit`（1–100，默认 50，硬上限 `auditLogListLimitMax=100`）、`action` / `outcome` / `actor` 精确过滤。响应含 `next_cursor`（页内最小 id，0 表示到底）。

### 4.5 保留策略（`internal/application/service/audit_log_retention.go`）

- 配置：`audit.retention_days`（YAML）/ `YUHENG_AUDIT_RETENTION_DAYS`（env 覆盖）；省略 `audit:` 段时默认 **90 天**；显式 0 表示禁用清扫（合规场景库外归档），负值在 config 校验时报错。
- `AuditLogRetentionRunner`：裸 `time.Ticker` 后台 goroutine（无 cron / asynq 依赖），启动延迟 10 分钟（避开迁移与启动流量），之后**每 24h** 执行一次 `Purge` → `DeleteOlderThan(now - retention_days)`（单条带索引 DELETE，30s 超时）。删除数量记 INFO，失败记 WARN（下轮再试）。由 `internal/container/container.go` 装配并注册 `ResourceCleaner` 优雅停止（`Stop` 幂等，未 Start 直接返回）。

## 5. 限流与登录保护（`internal/ratelimit` 与中间件）

### 5.1 通用滑动窗口限流器（`internal/ratelimit/limiter.go`）

- Redis 优先：Lua 脚本原子完成"剔除过期 ZSET 成员 → `ZCARD` 计数 → 未超限则 `ZADD` + `PEXPIRE`"，多实例共享预算；member 为 `<instanceID>:<ms>` 保证唯一。
- Redis 不可用（未配置或连接错误）时**自动降级**为进程内 `localLimiter`（`sync.Map` + 每 key 时间戳数组），`StartCleanup` 周期驱逐空 key。
- `max` 按每次 `Allow` 调用传入，同一 limiter 可对不同 key 用不同预算。

### 5.2 认证端点的 IP 限流（`internal/middleware/auth_ip_ratelimit.go`）

`AuthIPRateLimit(scope, limit)` 基于上面的限流器，按 `c.ClientIP()` 分桶，超限返回 429 并带 `Retry-After: 60`：

| 端点 | 每 IP 每分钟 |
| --- | --- |
| `POST /auth/login` | 30 |
| `POST /auth/register` | 10 |
| `POST /auth/switch-tenant` | 60 |
| 解锁带密码的公开文档页 | 10 |

配置了 Redis 时（`SetAuthRateLimitRedis`）预算跨实例共享，否则每个进程各自计数。

### 5.3 登录失败锁定（`internal/ratelimit/lockout.go`）

同一账号 15 分钟内登录失败 5 次即锁定 15 分钟（`internal/handler/auth.go` 的 `loginMaxFailures` / `loginFailureWindow` / `loginLockDuration`）。这是防分布式猜密码的主要手段，IP 限流只限制单一来源的量。进程内表有条目上限，防止用随意构造的邮箱把内存撑爆。

### 5.4 邀请链接端点（`internal/middleware/auth_public_ratelimit.go`）

`PublicAuthRateLimit()` 保护未认证的邀请链接端点（`/auth/invitations/lookup`、`/auth/register-by-invite`）：进程内滑动窗口，每 IP **30 次/分钟**（跨两个端点共享桶），超限返回 429（`ErrTooManyRequests`）。纯本地实现（低流量端点），注释中明确水平扩展时应换用 `internal/ratelimit` 的 Redis 版。

## 6. 健康检查

`internal/router/router.go` 在认证中间件之前注册两个无需认证的探针：

| 路径 | 含义 | 返回 |
| --- | --- | --- |
| `GET /health` | 存活（liveness）：进程在运行，不检查依赖 | 200 `{"status":"ok"}` |
| `GET /ready` | 就绪（readiness）：数据库可 ping、Redis（配置了时）可 ping、启动时的迁移成功且没有 dirty 版本（`internal/router/ready.go`，每项检查 2s 超时） | 全部正常 200 `{"status":"ok","checks":{...}}`；否则 503 `{"status":"unavailable","checks":{...}}`，只给出失败项的名字，原因写日志 |

编排系统按 `/ready` 分流、按 `/health` 重启：把重启绑到数据库故障上只会把依赖事故变成重启循环。docker compose 的 app 健康检查用的是 `/ready`（frontend 与 collab 等它就绪才启动），Helm chart 的 liveness 用 `/health`、readiness 用 `/ready`。`langfuse.shouldTrace` 不追踪这两个路径。进程 uptime 由 `internal/runtime/server.go` 的 `MarkServerStarted`/`ServerUptime` 提供给运维面板。

## 7. 模型引用统计（`internal/application/repository/model_usage.go`）

该文件提供的是**模型引用（usage-by-reference）查询**，即回答"哪些资源正在使用某个模型"，用于删除模型前的依赖保护，而非 token 用量计费：

- `scopeKnowledgeBasesByModelID`：匹配 `knowledge_bases` 中任一模型绑定字段 —— `embedding_model_id`、`summary_model_id`、`image_processing_config.model_id`、`vlm_config.model_id`、`asr_config.model_id`、`wiki_config.synthesis_model_id`（用 Postgres 的 `->>` JSON 操作符取值）。
- 消费方：`knowledgebase.go` 仓储的 `CountByModelID`，被 `internal/application/service/model.go` 的删除守卫调用（KB 引用计数 > 0 时阻止删除模型）。

token 级别的模型用量则由 Langfuse Generation 的 `usage_details`（`TokenUsage`：input/output/total/cache_*）上报，在 Langfuse UI 中按模型 / 用户（`tenant:<id>`）/ 会话聚合查看。

## 8. 运维速查

| 想知道… | 去哪里 |
| --- | --- |
| 某次请求全链路发生了什么 | 用响应头 `X-Request-ID` grep 应用日志；开启 `LLM_DEBUG_LOG` 后看 `llm_debug/<request_id>.log` |
| 一次聊天/解析的 LLM 调用树与 token 消耗 | Langfuse UI（trace 名 `POST /api/v1/knowledge-chat` 或 `asynq.document:process`） |
| 谁在什么时候改了什么 | 空间审计 `/tenants/:id/audit-log`；KB 活动 `/knowledge-bases/:id/activity`；平台审计 `/system/admin/audit-log` |
| 为什么某文档一直失败 | `task_dead_letters` 表（scope=knowledge/knowledge_base）+ 运行时面板 archived 任务的 `last_error` |
| 服务是否存活 / 就绪 | `GET /health`（200 `{"status":"ok"}`）；`GET /ready`（503 时看 `checks` 里哪一项失败） |
| 配置是否按预期加载 | 启动日志 `[startup-env]` 横幅（`internal/runtime/startup.go`，敏感值只显示长度） |

## 实现参考

想读源码时按下表定位（路径相对仓库根目录）：

| 能力 | 源码路径 |
| --- | --- |
| 应用日志 | `internal/logger/logger.go` |
| LLM 调用调试日志 | `internal/logger/llm_logger.go` |
| 请求日志 / RequestID 中间件 | `internal/middleware/logger.go` |
| Langfuse 追踪（OTel SDK） | `internal/tracing/langfuse/`（`config.go`、`manager.go`、`exporter.go`、`tracer.go`、`middleware.go`、`asynq.go`、`events.go`、`retrieval_obs.go`、`context.go`） |
| 跨进程 trace 载体 | `internal/types/tracing.go` |
| 审计日志 handler / service / repo | `internal/handler/audit_log.go`、`internal/application/service/audit_log.go`、`internal/application/repository/audit_log.go` |
| 审计保留策略 | `internal/application/service/audit_log_retention.go`、`internal/config/config.go`（`applyAuditDefaults`） |
| 审计动作 / 模型 | `internal/types/audit_log.go` |
| 限流与登录锁定 | `internal/ratelimit/limiter.go`、`internal/ratelimit/lockout.go`、`internal/middleware/auth_ip_ratelimit.go`、`internal/middleware/auth_public_ratelimit.go` |
| 健康检查 | `internal/router/router.go`（`GET /health`）、`internal/router/ready.go`（`GET /ready`） |
| 模型引用统计 | `internal/application/repository/model_usage.go` |
