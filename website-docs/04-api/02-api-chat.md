# API 参考：会话、消息与聊天

路由注册：`internal/router/routes_chat.go` 的 `RegisterSessionRoutes`、`RegisterMessageFeedbackRoutes`、`RegisterChatRoutes`、`RegisterMessageRoutes`。Handler：`internal/handler/session/`（handler.go、qa.go、stream.go、title.go、temporary_document.go）、`internal/handler/message.go`、`internal/handler/message_suggestion.go`、`internal/handler/message_feedback.go`。

会话只是对话容器，保存标题、描述、置顶状态等基础信息；知识库范围、模型、检索设置都随每次问答请求传入，不存在会话里。会话为“用户私有”资源，handler 内部强制归属校验；路由层为 Viewer+。API key：会话/聊天/回答反馈路由需 `chat` capability（或 full-access）；消息搜索需 `message_history`；知识检索需 `retrieve`。回答反馈虽然对 `chat` key 开放路由，服务层要求调用者是登录用户，API key 调用返回 403。

## 会话（/api/v1/sessions）

### POST /api/v1/sessions

用途：创建会话。Handler: `internal/handler/session/handler.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `title` | string | 否 | 标题 |
| `description` | string | 否 | 描述 |

响应：201 `{"success":true,"data":{Session}}`（`id,title,description,tenant_id,user_id,is_pinned,last_request_state,created_at,...`）

```bash
curl -X POST $BASE/api/v1/sessions -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"title":"新对话"}'
```

### GET /api/v1/sessions

用途：会话列表。Handler: `internal/handler/session/handler.go`

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `page` / `page_size` | int | 否 | 分页，默认 1 / 20，`page_size` 上限 1000 |
| `keyword` | string | 否 | 标题模糊搜索（`ILIKE %keyword%`） |
| `source` | string | 否 | 来源过滤：留空或 `web` 为调用者自己在 Web 控制台的会话；`api` 列出本工作区所有经 API key 创建的会话（需 Admin+） |

响应：200 `{"success":true,"data":[SessionListItem],"total","page","page_size"}`。列表项总是带置顶状态（`is_pinned`、`pinned_at`）。

```bash
curl "$BASE/api/v1/sessions?page=1" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/:id

用途：会话详情。

响应：200 `{"success":true,"data":{Session}}`（置顶时带 `pinned_at`）；会话不存在返回 404。

```bash
curl $BASE/api/v1/sessions/s-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/sessions/:id

用途：更新会话（标题/描述/置顶）。请求体：`title`、`description`、`is_pinned`（均可选）。

响应：200 `{"success":true,"data":{Session}}`

```bash
curl -X PUT $BASE/api/v1/sessions/s-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"title":"重命名"}'
```

### DELETE /api/v1/sessions/:id

用途：删除会话。

响应：200 `{"success":true,"message":"Session deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/sessions/s-1 -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/sessions/batch

用途：批量删除会话。请求体：`{"ids":["s-1"],"delete_all":false}`（二选一：`ids` 或 `delete_all:true`）。`delete_all:true` 删除调用者在当前工作区的全部会话，并忽略 `ids`。

响应：200 `{"success":true,"message":"Sessions deleted successfully"}`；`delete_all` 时 `message` 为 `All sessions deleted successfully`。

```bash
curl -X DELETE $BASE/api/v1/sessions/batch -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"ids":["s-1","s-2"]}'
```

### DELETE /api/v1/sessions/:id/messages

用途：清空会话里的全部消息，会话本身保留。聊天历史知识库里对应的条目会在后台异步清理。

响应：200 `{"success":true,"message":"Session messages cleared successfully"}`

```bash
curl -X DELETE $BASE/api/v1/sessions/s-1/messages -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/sessions/:session_id/generate_title

用途：根据上下文消息生成会话标题。Handler: `internal/handler/session/title.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `messages` | []Message | 是（`binding:"required"`） | 用作上下文的消息 |

响应：200 `{"success":true,"data":"生成的标题"}`

```bash
curl -X POST $BASE/api/v1/sessions/s-1/generate_title -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"messages":[{"role":"user","content":"介绍下产品"}]}'
```

### POST /api/v1/sessions/:session_id/stop

用途：停止正在生成的回答：服务端向该消息的流写入一个 `stop` 事件，由正在输出的 SSE 连接感知后取消生成。只允许会话所有者本人停止，工作区管理员能查看 API key 会话，但不能中断它。Handler: `internal/handler/session/stream.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `message_id` | string | 是（`binding:"required"`） | 助手消息 ID |

响应：200 `{"success":true,"message":"Generation stopped"}`；消息已经生成完毕时为 200 `{"success":true,"message":"Message already completed"}`。消息不属于该会话或会话不在当前工作区返回 403，消息或会话不存在返回 404。

```bash
curl -X POST $BASE/api/v1/sessions/s-1/stop -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"message_id":"m-1"}'
```

### POST /api/v1/sessions/:session_id/pin 与 DELETE /api/v1/sessions/:id/pin

用途：置顶 / 取消置顶会话。无请求体。Handler: `internal/handler/session/handler.go`

响应：200 `{"success":true,"is_pinned":true|false}`；置顶是按用户记录的，会话不存在或对当前用户不可见返回 404。

```bash
curl -X POST $BASE/api/v1/sessions/s-1/pin -H "Authorization: Bearer $TOKEN"
curl -X DELETE $BASE/api/v1/sessions/s-1/pin -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/continue-stream/:session_id

用途：断线续传活跃流（重放历史事件 + 100ms 轮询新增量）。Handler: `internal/handler/session/stream.go`

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `message_id` | string | 是 | 要续传的助手消息 ID，通常取 `/messages/:session_id/load` 返回中 `is_completed=false` 的那条 |

响应：200 SSE（`text/event-stream`，事件格式与 `knowledge-chat` 相同，见总览“流式接口协议”）：先回放该消息已产生的全部事件，再继续推送，直到 `complete`。消息记录不存在返回 404 `Incomplete message not found`，流里已没有事件返回 404 `No stream events found`。

```bash
curl -N "$BASE/api/v1/sessions/continue-stream/s-1?message_id=m-1" -H "Authorization: Bearer $TOKEN"
```

## 会话附件（临时文档）

Handler: `internal/handler/session/temporary_document.go`

### POST /api/v1/sessions/:session_id/attachments

用途：上传会话级临时文档（异步解析）。multipart 字段：`file`（必填）、`parser_engine`（可选，指定解析引擎）。

响应：202 `{"success":true,"data":{TemporaryDocument}}`（`id,session_id,file_name,file_type,file_size,status(uploaded/processing/ready/failed),resource_ref,...`）

```bash
curl -X POST $BASE/api/v1/sessions/s-1/attachments -H "Authorization: Bearer $TOKEN" -F 'file=@notes.pdf'
```

### GET /api/v1/sessions/:id/attachments

用途：附件列表。

响应：200 `{"success":true,"data":[TemporaryDocument]}`

```bash
curl $BASE/api/v1/sessions/s-1/attachments -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/:id/attachments/:attachment_id

用途：附件详情（含解析状态）。

响应：200 `{"success":true,"data":{TemporaryDocument}}`

```bash
curl $BASE/api/v1/sessions/s-1/attachments/a-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/:id/attachments/:attachment_id/preview

用途：附件原文件预览。

响应：200 文件流（`Content-Disposition: inline|attachment`，`Cache-Control: private`）。

```bash
curl $BASE/api/v1/sessions/s-1/attachments/a-1/preview -H "Authorization: Bearer $TOKEN" -o preview.pdf
```

### DELETE /api/v1/sessions/:id/attachments/:attachment_id

用途：删除附件。

响应：204 No Content

```bash
curl -X DELETE $BASE/api/v1/sessions/s-1/attachments/a-1 -H "Authorization: Bearer $TOKEN"
```

## 回答建议（Suggestions）

Handler: `internal/handler/message_suggestion.go`

主回答结束后，服务端在后台生成追问建议，不阻塞 SSE 的 `complete` 事件。建议集按工作区、助手消息、位置、配置快照与语言持久化并去重。用户点击某条建议时，客户端先上报 `click` 事件，再在下一次 `knowledge-chat` 请求里带上 `suggestion_attribution:{suggestion_set_id,question_id}`，服务端会校验归属。

### GET /api/v1/sessions/:id/messages/:message_id/suggestions

用途：读取某助手消息的追问建议。

响应：200 `{"success":true,"data":{MessageSuggestionSet}}`（`status(generating/ready/suppressed/failed),questions:[{id,text,category,source,knowledge_base_ids}],allow_regenerate,...`）

```bash
curl $BASE/api/v1/sessions/s-1/messages/m-1/suggestions -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/sessions/:session_id/messages/:message_id/suggestions

用途：确保生成建议（幂等触发）。请求体：`{"regenerate":true}`（可选，强制重新生成）。

响应：200（就绪）或 202（生成中）`{"success":true,"data":{MessageSuggestionSet|null}}`

```bash
curl -X POST $BASE/api/v1/sessions/s-1/messages/m-1/suggestions -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{}'
```

### POST /api/v1/sessions/:session_id/suggestion-events

用途：上报建议交互事件（埋点）。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `suggestion_set_id` | string | 是（`binding:"required"`） | 建议集 ID |
| `question_id` | string | 否 | click/regenerate 时必填 |
| `event_type` | string | 是（`binding:"required"`） | `impression/click/dismiss/regenerate` |

响应：204 No Content

```bash
curl -X POST $BASE/api/v1/sessions/s-1/suggestion-events -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"suggestion_set_id":"ss-1","event_type":"impression"}'
```

## 回答反馈（Feedback）

Handler: `internal/handler/message_feedback.go`，服务 `internal/application/service/message_feedback.go`。反馈属于会话所有者：路由层 Viewer+，服务层校验会话归调用者所有，并要求调用者是登录用户（API key 没有“某个人”的身份，调用返回 403）。

“没帮助”的反馈会作为“回答被反馈有误”（`disputed`）出现在该回答所引用文档的知识健康里，交给文档负责人处理，详见 [知识健康](../03-features/22-knowledge-health.md)。只对本工作区的文档生效。

### GET /api/v1/sessions/:id/feedback

用途：当前用户对该会话中各条回答的反馈。

响应：200 `{"success":true,"data":{"<message_id>":{message_id,rating,comment,share_question,updated_at}}}`，以消息 ID 为键；没有反馈的回答不出现。

```bash
curl $BASE/api/v1/sessions/s-1/feedback -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/sessions/:id/messages/:message_id/feedback

用途：评价一条回答，或撤回评价。只能评价 `role=assistant` 的消息。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `rating` | string | 否 | `up`（有帮助）、`down`（没帮助），空字符串表示撤回 |
| `comment` | string | 否 | 哪里不对，最多 1000 字符；仅 `down` 时保存，`up` 时忽略 |
| `share_question` | bool | 否 | 为 `true` 时，知识健康里的该条反馈附上对应的提问；仅 `down` 时生效 |

响应：200 `{"success":true,"data":{message_id,rating,comment,share_question,updated_at}}`；撤回时 `data` 为 `null`。`rating` 取值非法或意见过长返回 400，消息不存在返回 404。

```bash
curl -X PUT $BASE/api/v1/sessions/s-1/messages/m-1/feedback -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"rating":"down","comment":"引用的价格表已过期","share_question":true}'
```

## 聊天与检索

Handler: `internal/handler/session/qa.go`。API key：聊天需 `chat`/full；`knowledge-search` 需 `retrieve`/full。

### POST /api/v1/knowledge-chat/:session_id

用途：知识库问答（SSE 流式）。

请求体（`CreateKnowledgeQARequest`，`internal/handler/session/types.go`）：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `query` | string | 是（`binding:"required"`） | 用户问题 |
| `knowledge_base_ids` | []string | 否 | 检索的 KB |
| `knowledge_ids` | []string | 否 | 限定知识文件 |
| `web_search_enabled` | bool | 否 | 联网搜索 |
| `summary_model_id` | string | 否 | 总结模型 |
| `tag_ids` | []string | 否 | 标签过滤 |
| `mentioned_items` | []object | 否 | @提及项：`id`、`name`、`type`（`kb` / `file` / `tag`）、`kb_type`（`document` / `faq`，仅 `type=kb`）、`kb_id` 与 `kb_name`（文件或标签所属知识库） |
| `disable_title` | bool | 否 | 禁用自动标题 |
| `images` | []object | 否 | 图片（`data` base64 / `url` / `caption`） |
| `attachment_uploads` | []object | 否 | 内联附件（`data` base64、`file_name`、`file_size`） |
| `attachment_ids` | []string | 否 | 已上传的会话附件 ID |
| `channel` | string | 否 | 来源渠道，如 `web`、`api` |
| `suggestion_attribution` | object | 否 | 点击建议的归因信息 |

响应：200 SSE 流，`event: message` + `data: StreamResponse`（见总览），以 `complete` 事件结束。

```bash
curl -N -X POST $BASE/api/v1/knowledge-chat/s-1 -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"query":"退款政策是什么?","knowledge_base_ids":["kb-1"]}'
```

流的片段示例（省略了开头的 `agent_query` 与进度事件）：

```
event: message
data: {"id":"3475c004-...","response_type":"references","content":"","done":false,"knowledge_references":[{"id":"c8347bef-...","content":"...","knowledge_id":"a6790b93-...","chunk_index":0,"knowledge_title":"退款政策.pdf","score":0.82,"match_type":3,"chunk_type":"text","knowledge_filename":"退款政策.pdf"}]}

event: message
data: {"id":"3475c004-...","response_type":"answer","content":"根据退款政策，","done":false}

event: message
data: {"id":"3475c004-...","response_type":"answer","content":"","done":true}
```

### POST /api/v1/knowledge-search

用途：无会话知识检索（非流式）。Handler: `internal/handler/session/qa.go` 的 `SearchKnowledge`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `query` | string | 是（`binding:"required"`） | 查询 |
| `knowledge_base_ids` | []string | 否 | 检索的 KB |
| `knowledge_ids` | []string | 否 | 限定文件 |
| `tag_ids` | []string | 否 | 标签过滤 |
| `mentioned_items` | []object | 否 | 带 KB 范围的标签提及 |

`knowledge_base_ids`、`knowledge_ids`、带知识库范围的标签至少要给一种，否则返回 400。只给 `knowledge_ids` 时直接在这些文件里检索。

响应：200 `{"success":true,"data":[SearchResult]}`，只返回检索结果，不经过 LLM 总结。`SearchResult` 主要字段：

| 字段 | 说明 |
| --- | --- |
| `id` / `content` | 分块 ID 与命中文本 |
| `knowledge_id` / `knowledge_title` / `knowledge_filename` | 所属知识及其标题、文件名 |
| `knowledge_base_id` | 所属知识库 |
| `knowledge_source` / `knowledge_channel` | 知识来源类型与入库渠道 |
| `chunk_index` / `start_at` / `end_at` / `seq` | 分块序号、在源文档中的字符偏移、排序号 |
| `score` / `match_type` | 最终得分与命中方式 |
| `chunk_type` / `parent_chunk_id` / `sub_chunk_id` | 分块类型（`text` / `image` / `summary` 等）与父子分块关系 |
| `image_info` | 图片分块的附加信息（JSON 字符串） |
| `metadata` | 分块元数据 |
| `matched_content` | FAQ 命中时实际匹配到的问题 |

```bash
curl -X POST $BASE/api/v1/knowledge-search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"query":"部署要求","knowledge_base_ids":["kb-1"]}'
```

## 消息（/api/v1/messages）

Handler: `internal/handler/message.go`

### POST /api/v1/messages/search

用途：聊天历史搜索。权限：Viewer+；API key `message_history`/full。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `query` | string | 是（`binding:"required"`） | 查询 |
| `mode` | string | 否 | `keyword/vector/hybrid`（默认 hybrid） |
| `limit` | int | 否 | 默认 20 |
| `session_ids` | []string | 否 | 限定会话 |

响应：200 `{"success":true,"data":{"total":N,"items":[{request_id,session_id,session_title,query_content,answer_content,score,match_type,created_at}]}}`。同一 `request_id` 的提问与回答合并为一项。

```bash
curl -X POST $BASE/api/v1/messages/search -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"query":"报价"}'
```

### GET /api/v1/messages/chat-history-stats

用途：聊天历史索引统计。权限：Viewer+；API key `message_history`/full。

响应：200 `{"success":true,"data":{enabled,embedding_model_id,knowledge_base_id,knowledge_base_name,indexed_message_count,has_indexed_messages}}`

```bash
curl $BASE/api/v1/messages/chat-history-stats -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/messages/:session_id/load

用途：加载会话消息（时间游标向前翻页）。权限：Viewer+；API key `chat`/full。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `limit` | int | 否 | 默认 20 |
| `before_time` | string | 否 | RFC3339/RFC3339Nano 时间戳：传上一页最早一条消息的 `created_at` 往前翻，留空取最近的消息 |

响应：200 `{"success":true,"data":[Message]}`（`id,session_id,request_id,role,content,knowledge_references,mentioned_items,is_completed,is_fallback,images,attachments,usage,channel,model_id,created_at,...`）。同一轮的提问与回答共用 `request_id`；`is_fallback` 为 true 表示知识库没有命中、走了兜底回答；`is_completed=false` 的助手消息可以用 `continue-stream` 续传。

```bash
curl "$BASE/api/v1/messages/s-1/load?limit=20" -H "X-API-Key: $API_KEY"
```

### DELETE /api/v1/messages/:session_id/:id

用途：删除单条消息。权限：Viewer+（handler 校验会话归属）；API key `chat`/full。

响应：200 `{"success":true,"message":"Message deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/messages/s-1/m-1 -H "Authorization: Bearer $TOKEN"
```
