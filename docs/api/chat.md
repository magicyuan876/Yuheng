# 聊天功能 API

[返回目录](./README.md)

| 方法 | 路径                          | 描述                     |
| ---- | ----------------------------- | ------------------------ |
| POST | `/knowledge-chat/:session_id` | 基于知识库的问答         |
| POST | `/knowledge-search`           | 基于知识库的搜索知识     |
| GET  | `/sessions/:session_id/messages/:message_id/suggestions` | 获取已生成的回答后推荐 |
| POST | `/sessions/:session_id/messages/:message_id/suggestions` | 确保生成或换一批推荐 |
| POST | `/sessions/:session_id/suggestion-events` | 上报曝光、点击、关闭事件 |

> **变更说明**：`/agent-chat/:session_id` 端点已随 Agent 引擎移除，现在返回 `404`。需要 Agent 编排能力的场景，请由外部 Agent 通过 REST API / Go SDK / CLI / MCP server 调用 Yuheng 的检索与问答能力。

## POST `/knowledge-chat/:session_id` - 基于知识库的问答

基于知识库的 RAG 问答，支持 SSE 流式响应。

**查询参数**：

| 参数 | 取值 | 说明 |
|------|------|------|
| `resource_urls` | `handle`（默认）/ `public` | `public` 让答案与引用里的图片直接返回可加载的 http(s) 链接，省去逐个调用 `/files` 代理。详见[文件与图片引用](../../website-docs/03-features/21-file-access.md) |

同样适用于下面的 `/knowledge-search` 与 `/sessions/continue-stream/:session_id`。

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 查询文本 |
| `knowledge_base_ids` | string[] | 否 | 知识库 ID 列表 |
| `knowledge_ids` | string[] | 否 | 知识文件 ID 列表，指定具体文件进行检索 |
| `summary_model_id` | string | 否 | 覆盖默认的摘要模型 ID |
| `mentioned_items` | object[] | 否 | @提及的知识库和文件列表 |
| `disable_title` | bool | 否 | 是否禁用自动标题生成（默认 false） |
| `images` | object[] | 否 | 附带的图片（base64 格式） |
| `channel` | string | 否 | 来源渠道标识：`web`、`api`、`browser_extension` |
| `suggestion_attribution` | object | 否 | 用户从推荐问题发起本轮时传入 `{suggestion_set_id, question_id}`；服务端会校验归属 |

**请求**:

```curl
curl --location 'http://localhost:8080/api/v1/knowledge-chat/ceb9babb-1e30-41d7-817d-fd584954304b' \
--header 'X-API-Key: sk-xxxxx' \
--header 'Content-Type: application/json' \
--data '{
    "query": "彗尾的形状",
    "knowledge_base_ids": ["kb-00000001"]
}'
```

**响应格式**:
服务器端事件流（Server-Sent Events，Content-Type: text/event-stream）

**响应**:

```
event: message
data: {"id":"3475c004-0ada-4306-9d30-d7f5efce50d2","response_type":"references","content":"","done":false,"knowledge_references":[{"id":"c8347bef-...","content":"彗星xxx。","knowledge_id":"a6790b93-...","chunk_index":0,"knowledge_title":"彗星.txt","score":4.04,"match_type":3,"chunk_type":"text","knowledge_filename":"彗星.txt"}]}

event: message
data: {"id":"3475c004-0ada-4306-9d30-d7f5efce50d2","response_type":"answer","content":"彗尾的形状主要表现为...","done":false,"knowledge_references":null}

event: message
data: {"id":"3475c004-0ada-4306-9d30-d7f5efce50d2","response_type":"answer","content":"","done":true,"knowledge_references":null}
```

## 回答后推荐问题

回答主消息完成后，服务端会异步生成推荐问题，不阻塞 SSE 的 `complete`/`done` 事件。生成结果按“空间、助手消息、位置、配置快照、语言”持久化并去重。

```http
POST /api/v1/sessions/{session_id}/messages/{message_id}/suggestions
Content-Type: application/json

{"regenerate": false}
```

状态包括 `generating`、`ready`、`suppressed`、`failed`。`ready` 时的每个问题都有稳定 `id`，点击后应先上报事件，并在下一次聊天请求中携带 `suggestion_attribution`。

```http
POST /api/v1/sessions/{session_id}/suggestion-events
Content-Type: application/json

{
  "suggestion_set_id": "...",
  "question_id": "...",
  "event_type": "click"
}
```

**mentioned_items 结构**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 知识库或文件 ID |
| `name` | string | 显示名称 |
| `type` | string | 类型：`kb`（知识库）或 `file`（文件） |
| `kb_type` | string | 知识库类型：`document` 或 `faq`（仅 `type=kb` 时） |

**images 结构**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `data` | string | base64 编码的图片数据（`data:image/png;base64,...`） |
