# Go SDK

Yuheng 官方 Go SDK 位于仓库的 `client/` 目录，是一个独立的 Go module，封装了 Yuheng 服务端 `/api/v1/*` 主要资源的操作与 SSE 流式对话能力。官方 CLI（`yuheng`）基于此 SDK 构建（`cli/go.mod` 通过 `replace` 指向仓库内的 `../client`）。Yuheng 当前为 0.1.0 预览版，`/api/v1` 在 0.x 版本之间可能变化，SDK 随之调整。

## 安装

SDK 的 module 路径定义在 `client/go.mod`：

```
module github.com/magicyuan876/yuheng/client

go 1.24.2
```

安装方式：

```bash
go get github.com/magicyuan876/yuheng/client
```

导入：

```go
import "github.com/magicyuan876/yuheng/client"
```

## 初始化与认证

核心类型与构造函数定义在 `client/client.go`。

### Client 结构

```go
type Client struct {
    baseURL       string
    httpClient    *http.Client
    streamTimeout time.Duration
    apiKey        string
    bearerToken   string
    tenantID      *uint64
}
```

通过 `NewClient(baseURL string, options ...ClientOption) *Client` 创建实例。默认的普通请求超时为 30 秒；流式（SSE）请求默认**无超时**，生命周期由 `context` 控制（除非显式调用 `WithTimeout`）。

### ClientOption 一览

| Option | 说明 |
|---|---|
| `WithAPIKey(key string)` | 设置长期有效的 API Key，以 `X-API-Key` 请求头发送 |
| `WithBearerToken(token string)` | 设置短期 JWT，以 `Authorization: Bearer <token>` 请求头发送（通常在 `Login` 成功后使用） |
| `WithToken(token string)` | **Deprecated**：`WithAPIKey` 的 v0.x 兼容别名，将在下个大版本移除 |
| `WithTimeout(timeout time.Duration)` | 同时设置普通请求与流式请求的超时上限 |
| `WithTransport(rt http.RoundTripper)` | 替换底层 `http.RoundTripper`（用于重试/埋点/签名等中间件）；传 `nil` 恢复 `http.DefaultTransport` |
| `WithTenantID(tenantID uint64)` | 在每个请求上附加 `X-Tenant-ID` 请求头，仅用于具备 `CanAccessAllTenants` 权限的跨租户显式访问 |

### 认证方式说明

SDK 支持两种凭证，可同时配置，HTTP 层 `X-API-Key` 优先：

- **API Key**（长期）：`WithAPIKey`，请求头 `X-API-Key`；
- **Bearer JWT**（短期）：`WithBearerToken`，请求头 `Authorization: Bearer <token>`，配合 `client/auth.go` 中的 `Login` / `RefreshToken` / `GetCurrentUser` 使用。

典型 JWT 登录流程（对应 `POST /api/v1/auth/login`）：

```go
c := client.NewClient("http://localhost:8080")
loginResp, err := c.Login(ctx, client.LoginRequest{Email: "user@example.com", Password: "..."})
if err != nil {
    return err
}
// LoginResponse.Token 是 JWT access token，RefreshToken 用于续期
authed := client.NewClient("http://localhost:8080",
    client.WithBearerToken(loginResp.Token))
```

### 租户（Tenant）与请求头注入

`applyAuthHeaders`（`client/client.go`）会在每个请求上自动注入：

- `X-API-Key` / `Authorization`（按配置）；
- `X-Request-ID`：从 `ctx.Value("RequestID")`（string 类型）读取，用于链路追踪；
- `X-Tenant-ID`：优先级为 context 中的 `"TenantID"` 值（支持 `uint64`、`*uint64`、数字字符串）> `WithTenantID` 的客户端级默认值。

单请求租户覆盖示例：

```go
tenantID := uint64(10000)
ctx := context.WithValue(context.Background(), "TenantID", &tenantID)
kb, err := apiClient.GetKnowledgeBase(ctx, kbID)
```

注意：JWT 与租户级 API Key 本身已携带租户身份，普通用户**不应**设置 `X-Tenant-ID`（服务端 auth 中间件会对携带该头的 bearer 请求执行跨租户校验，普通用户会得到 403）。

### Raw 逃生舱

`Client.Raw(ctx, method, path, body)`（Experimental）以客户端已配置的认证头直接发起任意 HTTP 请求，用于一次性集成与 `yuheng api` CLI 透传；有类型化方法时应优先使用类型化方法。

## 资源与方法总览

以下均为 `Client` 的公开方法（共 159 个），内部方法（`buildRequest`、`doRequest`、`doRequestStream`、`applyAuthHeaders`）不列入。

SDK 覆盖知识库、文档、检索与对话这条主线，以及租户、组织、模型等管理接口；以下服务端接口**没有**类型化方法，需要时用 `Raw` 调用：知识健康（`/knowledge-bases/:id/findings`、`/findings/assigned`、`/knowledge/:id/stewardship` 等，见[知识健康](../03-features/22-knowledge-health.md)）、答案反馈（`/sessions/:id/feedback`）、在线文档（`/docs/...`）、Wiki、数据源、向量存储与存储后端、Web 搜索 provider 管理、成员与邀请、审计日志、系统管理（`/system/admin/...`）。

### 认证 Auth — `client/auth.go`

| 方法 | 说明 |
|---|---|
| `Login` | 邮箱密码登录，返回 JWT access/refresh token（`POST /api/v1/auth/login`） |
| `GetCurrentUser` | 获取当前登录主体与租户信息（`GET /api/v1/auth/me`） |
| `RefreshToken` | 用 refresh token 换取新 access token |
| `ChangePassword` | 修改当前用户密码（`POST /api/v1/auth/change-password`）；成功后服务端吊销该用户所有会话，调用方应丢弃本地 token |

### 知识库 KnowledgeBase — `client/knowledgebase.go`

| 方法 | 说明 |
|---|---|
| `CreateKnowledgeBase` | 创建知识库 |
| `GetKnowledgeBase` | 获取知识库详情 |
| `ListKnowledgeBases` | 列出知识库 |
| `UpdateKnowledgeBase` | 更新知识库 |
| `DeleteKnowledgeBase` | 删除知识库 |
| `ClearKnowledgeBaseContents` | 清空知识库内容 |
| `HybridSearch` | 在知识库内混合检索（向量 + 关键词） |
| `TogglePinKnowledgeBase` | 置顶/取消置顶 |
| `ListMoveTargets` | 列出知识可迁移的目标知识库 |
| `CopyKnowledgeBase` | 复制知识库（异步任务） |
| `DuplicateKnowledgeBase` | 创建知识库副本（只复制设置，不复制内容） |
| `GetKBCloneProgress` | 查询复制任务进度 |

### 知识 Knowledge — `client/knowledge.go`

| 方法 | 说明 |
|---|---|
| `CreateKnowledgeFromFile` | 从本地文件上传创建知识（multipart，支持 metadata、多模态开关、自定义文件名、channel、解析配置覆盖） |
| `CreateKnowledgeFromURL` | 从 URL 创建知识 |
| `CreateManualKnowledge` / `UpdateManualKnowledge` | 创建/更新手写（manual）知识 |
| `GetKnowledge` | 获取知识详情 |
| `GetKnowledgeBatch` | 批量获取知识 |
| `ListKnowledge` | 分页列出知识 |
| `ListKnowledgeWithFilter` | 带过滤条件列出知识 |
| `FilterKnowledge` | 按关键词/文件类型跨库搜索知识 |
| `UpdateKnowledge` | 更新知识 |
| `DeleteKnowledge` | 删除知识 |
| `DownloadKnowledgeFile` | 下载知识原始文件到本地路径 |
| `OpenKnowledgeFile` | 以流方式打开知识原始文件（返回文件名 + `io.ReadCloser`） |
| `PreviewKnowledgeFile` | 预览知识文件（返回原始 `*http.Response`） |
| `ReparseKnowledge` | 重新解析知识 |
| `CancelKnowledgeParse` | 取消解析任务 |
| `GetKnowledgeProcessingSpans` | 获取知识处理链路 span（`GET /api/v1/knowledge/:id/spans`） |
| `UpdateImageInfo` | 更新图片信息 |
| `ListKnowledgeFolders` | 获取知识库的文件夹树 |
| `MoveKnowledgeToFolder` | 把知识移入文件夹（空路径表示移回根目录） |
| `RenameKnowledgeFolder` | 重命名文件夹（连同所有子路径） |
| `MoveKnowledge` | 跨知识库迁移知识 |
| `GetKnowledgeMoveProgress` | 查询迁移任务进度 |
| `BatchUpdateKnowledgeTags` | 批量更新知识标签 |

### 分块 Chunk — `client/chunk.go`

| 方法 | 说明 |
|---|---|
| `ListKnowledgeChunks` | 分页列出某个知识的 chunk，可按 chunk 类型过滤 |
| `UpdateChunk` | 更新 chunk 内容/启用状态 |
| `DeleteChunk` | 删除 chunk |
| `GetChunkByIDOnly` | 仅凭 chunk ID 获取 chunk |
| `DeleteGeneratedQuestion` | 删除 chunk 生成的问题。注意：该方法当前请求的是 `DELETE /api/v1/chunks/{chunkID}/delete-question`，而服务端注册的删除路由是 `DELETE /api/v1/chunks/by-id/:id/questions`，在修复前不要依赖它 |
| `DeleteChunksByKnowledgeID` | 删除某知识的全部 chunk |

### 会话 Session — `client/session.go`

| 方法 | 说明 |
|---|---|
| `CreateSession` | 创建会话 |
| `GetSession` | 获取会话 |
| `GetSessionsByTenant` | 分页列出会话 |
| `UpdateSession` | 更新会话 |
| `DeleteSession` | 删除会话 |
| `BatchDeleteSessions` | 批量删除会话 |
| `GenerateTitle` | 生成会话标题 |
| `KnowledgeQAStream` | 知识问答（SSE 流式，见下文） |
| `ContinueStream` | 续接进行中的流（断线重连场景） |
| `StopSession` | 停止某条 assistant 消息的生成 |
| `SearchKnowledge` | 知识检索（不经 LLM 总结，`POST /api/v1/knowledge-search`） |

### 消息 Message — `client/message.go`、`client/message_suggestion.go`

| 方法 | 说明 | 源文件 |
|---|---|---|
| `LoadMessages` | 按时间加载消息 | `client/message.go` |
| `GetRecentMessages` | 获取最近 N 条消息 | `client/message.go` |
| `GetMessagesBefore` | 获取某时间点之前的消息 | `client/message.go` |
| `SearchMessages` | 搜索历史消息 | `client/message.go` |
| `GetChatHistoryKBStats` | 聊天历史知识库统计 | `client/message.go` |
| `DeleteMessage` | 删除消息 | `client/message.go` |
| `EnsureMessageSuggestions` | 确保（可强制重新）生成推荐问题 | `client/message_suggestion.go` |
| `GetMessageSuggestions` | 获取消息的推荐问题 | `client/message_suggestion.go` |
| `RecordMessageSuggestionEvent` | 上报推荐问题点击/曝光事件 | `client/message_suggestion.go` |

`KnowledgeQAStream`、`ContinueStream`、`SearchKnowledge`、`LoadMessages` 接受可选的 `ResourceURLOptions`（`client/resource_urls.go`）：`ResourceURLModePublic` 让服务端把文件引用返回成无需 Yuheng 凭证即可加载的限时 HTTP(S) URL，默认的 `ResourceURLModeHandle` 返回 `resource://` 句柄，需经已认证的 `/files` 代理取回。

### 模型 Model — `client/model.go`

| 方法 | 说明 |
|---|---|
| `CreateModel` | 创建模型 |
| `GetModel` | 获取模型 |
| `ListModels` | 列出模型 |
| `UpdateModel` | 更新模型 |
| `DeleteModel` | 删除模型 |
| `ListModelProviders` | 按模型类型列出模型提供商 |

包级辅助函数 `AllModelTypes()`、`AllModelSources()` 返回模型类型与来源的全部枚举值。

### 租户 Tenant — `client/tenant.go`

| 方法 | 说明 |
|---|---|
| `CreateTenant` / `GetTenant` / `UpdateTenant` / `DeleteTenant` | 租户 CRUD |
| `ListTenants` | 列出当前用户可访问的租户 |
| `ListAllTenants` | 列出全部租户（跨租户权限） |
| `SearchTenants` | 搜索租户（分页，跨租户权限） |
| `ListTenantAPIKeys` / `CreateTenantAPIKey` / `UpdateTenantAPIKey` / `DeleteTenantAPIKey` | 租户 API Key 管理 |
| `GetTenantKV` / `UpdateTenantKV` | 读取/更新租户级 KV 配置 |
| `GetAPIPrincipalConfig` / `UpdateAPIPrincipalConfig` | API 主体配置 |
| `CreateAPIPrincipalTestToken` | 创建 API 主体测试 token |

### 组织与共享 Organization — `client/organization.go`

| 方法 | 说明 |
|---|---|
| `CreateOrganization` / `ListMyOrganizations` / `GetOrganization` / `UpdateOrganization` / `DeleteOrganization` | 组织 CRUD |
| `SearchOrganizations` / `PreviewOrganizationByInviteCode` | 搜索/邀请码预览组织 |
| `JoinOrganizationByInviteCode` / `SubmitJoinRequest` / `JoinByOrganizationID` / `LeaveOrganization` / `RequestRoleUpgrade` | 加入/退出/申请升级角色 |
| `GenerateInviteCode` / `SearchUsersForInvite` / `InviteMember` | 邀请成员 |
| `ListOrgMembers` / `UpdateMemberRole` / `RemoveMember` | 成员管理 |
| `ListJoinRequests` / `ReviewJoinRequest` | 加入申请审批 |
| `ShareKnowledgeBase` / `ListKBShares` / `UpdateSharePermission` / `RemoveKBShare` | 知识库共享 |
| `ListOrgShares` / `ListSharedKnowledgeBases` | 组织内共享的知识库、共享给我的知识库 |

### FAQ — `client/faq.go`

| 方法 | 说明 |
|---|---|
| `ListFAQEntries` | 分页列出 FAQ 条目 |
| `UpsertFAQEntries` | 批量新增/更新 FAQ 条目（异步导入） |
| `CreateFAQEntry` | 创建单条 FAQ |
| `GetFAQEntry` | 获取单条 FAQ |
| `UpdateFAQEntry` | 更新单条 FAQ |
| `AddSimilarQuestions` | 追加相似问 |
| `UpdateFAQEntryFieldsBatch` | 批量更新字段 |
| `UpdateFAQEntryTagBatch` | 批量更新标签 |
| `DeleteFAQEntries` | 批量删除 |
| `SearchFAQEntries` | 检索 FAQ |
| `ExportFAQEntries` | 导出为 CSV（返回 `[]byte`） |
| `GetFAQImportProgress` | 查询异步导入任务进度（含 dry run） |
| `UpdateLastFAQImportResultDisplayStatus` | 更新最近导入结果的展示状态 |

### 标签 Tag — `client/tag.go`

| 方法 | 说明 |
|---|---|
| `ListTags` | 列出标签 |
| `CreateTag` | 创建标签 |
| `UpdateTag` / `UpdateTagBySeqID` | 更新标签（按 ID / 按 seq ID） |
| `DeleteTag` / `DeleteTagBySeqID` | 删除标签（按 ID / 按 seq ID） |

### 初始化与模型检测 — `client/initialization.go`

| 方法 | 说明 |
|---|---|
| `GetInitializationConfig` / `InitializeByKB` / `UpdateKBConfig` / `SetKBModelConfig` | 知识库初始化与模型配置 |
| `CheckOllamaStatus` / `ListOllamaModels` / `CheckOllamaModels` | Ollama 状态与模型探测 |
| `DownloadOllamaModel` / `GetOllamaDownloadProgress` / `ListOllamaDownloadTasks` | Ollama 模型下载任务 |
| `CheckRemoteModel` / `TestEmbeddingModel` / `CheckRerankModel` / `TestMultimodalFunction` | 远程 LLM / Embedding / Rerank / 多模态连通性检测 |
| `ExtractTextRelations` | 文本关系抽取测试 |

### 系统 System — `client/system.go`

| 方法 | 说明 |
|---|---|
| `GetSystemInfo` | 获取系统信息（版本等） |
| `GetDeploymentCapabilities` | 获取部署能力快照（`GET /api/v1/system/capabilities`） |
| `ListParserEngines` / `CheckParserEngines` | 文档解析引擎列表/检测 |
| `ReconnectDocReader` | 重连 docreader 服务 |
| `GetStorageEngineStatus` / `CheckStorageEngine` | 存储引擎状态/检测 |

### 其他

| 方法 | 说明 | 源文件 |
|---|---|---|
| `StartEvaluation` / `GetEvaluationResult` | 发起评估任务 / 查询评估结果 | `client/evaluation.go` |
| `GetWebSearchProviders` | 列出可用 Web 搜索提供商 | `client/web_search.go` |
| `Raw` | 原始 HTTP 逃生舱（Experimental） | `client/client.go` |

## 流式对话（SSE）

SDK 的流式接口采用**回调（callback）机制**而非 channel：SDK 内部用 `bufio.Scanner` 逐行解析 SSE（`event:` / `data:` 前缀，空行分帧），每解析出一帧就调用一次回调；回调返回非 nil error 即中止流。SSE 行缓冲上限被提升到 4 MiB（`scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)`），避免 references 大帧触发 "token too long"。

流式请求走 `doRequestStream`（`client/client.go`），默认不受 30 秒超时约束，流生命周期由传入的 `ctx` 控制。

### 知识问答流：`KnowledgeQAStream`（`client/session.go`）

```go
func (c *Client) KnowledgeQAStream(
    ctx context.Context,
    sessionID string,
    request *KnowledgeQARequest,
    callback func(*StreamResponse) error,
) error
```

每帧 `StreamResponse` 携带 `ResponseType`、增量 `Content`、结束标记 `Done`、引用来源 `KnowledgeReferences`，以及 `SessionID`、`AssistantMessageID`、`Data` 等字段。SDK 定义的 `ResponseType` 常量有 `answer`、`references`、`thinking`、`tool_call`、`tool_result`、`error`、`reflection`、`session_title`、`agent_query`、`complete`（`agent_query` 是流的首个事件名，携带 session_id 与 assistant_message_id，与已移除的 Agent 功能无关）。

`KnowledgeQARequest` 的字段有 `Query`、`KnowledgeBaseIDs`、`KnowledgeIDs`、`WebSearchEnabled`、`SummaryModelID`（覆盖会话默认模型）、`DisableTitle`（不自动生成标题）、`Images`（多模态图片，base64）与 `Channel`：

```go
err := apiClient.KnowledgeQAStream(ctx, session.ID, &client.KnowledgeQARequest{
    Query:            "介绍一下 Yuheng",
    KnowledgeBaseIDs: []string{"kb-1"},
}, func(ev *client.StreamResponse) error {
    if ev.ResponseType == "answer" {
        fmt.Print(ev.Content)
    }
    return nil
})
```

### 断线续接：`ContinueStream`（`client/session.go`）

`ContinueStream(ctx, sessionID, messageID, callback)` 以 `GET /api/v1/sessions/continue-stream/{sessionID}?message_id=...` 续接服务端仍在生成的流，回调机制与 `KnowledgeQAStream` 相同；配合 `StopSession(ctx, sessionID, messageID)` 可中止生成。

## 错误处理

### HTTP 层：`APIError`（`client/client.go`）

所有非 2xx 响应被封装为 `*APIError`，用 `errors.As` 按 HTTP 状态码或服务端结构化错误码分支：

```go
var apiErr *client.APIError
if errors.As(err, &apiErr) {
    switch {
    case apiErr.StatusCode == 404:
        // 资源不存在
    case apiErr.Code == client.ServerErrUnauthorized: // 1001
        // 触发重新登录
    }
}
```

`Code` 为响应体 `{"code":N}` 中的结构化错误码，包内提供常量 `ServerErrBadRequest`(1000) 至 `ServerErrValidation`(1010)。`Error()` 保持 `"HTTP error <status>: <body>"` 的旧格式以兼容字符串匹配的消费者。

### 流层：`SSEStreamError`（`client/stream_errors.go`）

当服务端在 SSE 流上发出终止错误帧（`response_type=error, done=true`）时，SDK 会**先把该帧交给回调**，然后返回 `*SSEStreamError`：

```go
type SSEStreamError struct {
    Content string // 错误帧内容
}
```

判断方式（两者等价，推荐前者）：

```go
// 方式一：哨兵错误（SSEStreamError.Unwrap() 返回它）
if errors.Is(err, client.ErrSSEStreamTerminal) { ... }

// 方式二：辅助函数（兼容旧版 fmt.Errorf("SSE stream error: ...") 链）
if client.IsSSEStreamError(err) { ... }
```

## 日志与链路追踪

`client/log.go` 提供基于 `log/slog` 的 SDK 内部调试日志，默认写入 `io.Discard`（对使用方完全静默）。嵌入方可在启动时调用：

```go
client.SetDebugLevel("debug") // "debug"/"info"/"warn"；其他值（含 "error"、""）静默
```

日志输出到 stderr，包含 SSE 逐行解析、请求失败等 trace 信息。该函数**非并发安全**，须在任何 SDK 调用发起前调用一次。

链路追踪方面，在 context 中放入 `"RequestID"`（string），SDK 会自动作为 `X-Request-ID` 请求头发送（见 `client/client.go` 的 `applyAuthHeaders`）：

```go
ctx := context.WithValue(context.Background(), "RequestID", "req-20260727-0001")
```

## 完整示例

以下示例改编自 `client/example.go` 中的真实代码。

### 示例一：创建知识库并上传文件

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/magicyuan876/yuheng/client"
)

func main() {
    apiClient := client.NewClient(
        "http://localhost:8080",
        client.WithAPIKey("your-api-key"),
        client.WithTimeout(30*time.Second),
    )

    // 创建知识库
    kb := &client.KnowledgeBase{
        Name:        "Test Knowledge Base",
        Description: "This is a test knowledge base",
        ChunkingConfig: client.ChunkingConfig{
            ChunkSize:    500,
            ChunkOverlap: 50,
            Separators:   []string{"\n\n", "\n", ". ", "? ", "! "},
        },
        EmbeddingModelID: "embedding_model_id",
        SummaryModelID:   "summary_model_id",
    }
    createdKB, err := apiClient.CreateKnowledgeBase(context.Background(), kb)
    if err != nil {
        fmt.Printf("Failed to create knowledge base: %v\n", err)
        return
    }
    fmt.Printf("Knowledge base created: ID=%s, Name=%s\n", createdKB.ID, createdKB.Name)

    // 上传文件创建知识
    metadata := map[string]string{"source": "local", "type": "document"}
    knowledge, err := apiClient.CreateKnowledgeFromFile(
        context.Background(), createdKB.ID, "path/to/sample.pdf",
        metadata, nil, "", "", nil)
    if err != nil {
        fmt.Printf("Failed to upload knowledge file: %v\n", err)
        return
    }
    fmt.Printf("File uploaded: Knowledge ID=%s, Title=%s\n", knowledge.ID, knowledge.Title)
}
```

### 示例二：创建会话并进行流式知识问答

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "strings"

    "github.com/magicyuan876/yuheng/client"
)

func main() {
    apiClient := client.NewClient("http://localhost:8080",
        client.WithAPIKey("your-api-key"))

    // 创建会话
    session, err := apiClient.CreateSession(context.Background(), &client.CreateSessionRequest{
        Title:       "Test Session",
        Description: "A test session for knowledge Q&A",
    })
    if err != nil {
        fmt.Printf("Failed to create session: %v\n", err)
        return
    }

    // 流式问答：累积答案与引用
    question := "What is artificial intelligence?"
    var answer strings.Builder
    var references []*client.SearchResult

    err = apiClient.KnowledgeQAStream(context.Background(),
        session.ID,
        &client.KnowledgeQARequest{Query: question},
        func(response *client.StreamResponse) error {
            if response.ResponseType == client.ResponseTypeAnswer {
                answer.WriteString(response.Content)
            }
            if response.Done && len(response.KnowledgeReferences) > 0 {
                references = response.KnowledgeReferences
            }
            return nil
        })
    if err != nil {
        // 区分 SSE 终止错误帧与其他错误
        if errors.Is(err, client.ErrSSEStreamTerminal) {
            fmt.Printf("Stream terminated by server error: %v\n", err)
        } else {
            fmt.Printf("Q&A failed: %v\n", err)
        }
        return
    }
    fmt.Printf("Answer: %s\n", answer.String())
    for i, ref := range references {
        fmt.Printf("Reference %d: %s\n", i+1, ref.Content)
    }
}
```

### 示例三：历史消息与 Chunk 管理及资源清理

```go
package main

import (
    "context"
    "fmt"

    "github.com/magicyuan876/yuheng/client"
)

func main() {
    apiClient := client.NewClient("http://localhost:8080",
        client.WithAPIKey("your-api-key"))
    ctx := context.Background()

    // 获取最近 10 条会话消息
    sessionID := "your-session-id"
    messages, err := apiClient.GetRecentMessages(ctx, sessionID, 10)
    if err != nil {
        fmt.Printf("Failed to get session messages: %v\n", err)
    } else {
        for i, msg := range messages {
            fmt.Printf("%d. Role: %s, Content: %s\n", i+1, msg.Role, msg.Content)
        }
    }

    // 管理知识 chunk：分页列出并更新第一个
    knowledgeID := "your-knowledge-id"
    chunks, total, err := apiClient.ListKnowledgeChunks(ctx, knowledgeID, 1, 10)
    if err != nil {
        fmt.Printf("Failed to get knowledge chunks: %v\n", err)
    } else {
        fmt.Printf("Knowledge has %d chunks, retrieved %d\n", total, len(chunks))
        if len(chunks) > 0 {
            updated, err := apiClient.UpdateChunk(ctx, knowledgeID, chunks[0].ID,
                &client.UpdateChunkRequest{
                    Content:   "Updated chunk content - " + chunks[0].Content,
                    IsEnabled: true,
                })
            if err != nil {
                fmt.Printf("Failed to update chunk: %v\n", err)
            } else {
                fmt.Printf("Chunk updated: ID=%s\n", updated.ID)
            }
        }
    }

    // 清理资源
    if err := apiClient.DeleteSession(ctx, sessionID); err != nil {
        fmt.Printf("Failed to delete session: %v\n", err)
    }
    if err := apiClient.DeleteKnowledge(ctx, knowledgeID); err != nil {
        fmt.Printf("Failed to delete knowledge: %v\n", err)
    }
}
```

## 参考源码

- 客户端核心与错误类型：`client/client.go`
- 认证：`client/auth.go`
- 流式问答：`client/session.go`
- 流式错误：`client/stream_errors.go`
- 日志：`client/log.go`
- 完整用法示例：`client/example.go`
