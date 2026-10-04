# API 参考：知识库与知识

路由注册：`internal/router/routes_knowledge.go` 的 `RegisterKnowledgeBaseRoutes`、`RegisterKnowledgeRoutes`、`RegisterKnowledgeFindingRoutes`、`RegisterKnowledgeStewardshipRoutes`。Handler：`internal/handler/knowledgebase.go`、`internal/handler/knowledge.go`、`internal/handler/knowledge_finding.go`、`internal/handler/knowledge_stewardship.go`。

权限速记：知识库只能从拥有它的工作区访问——KB 访问守卫对别的工作区的知识库与不存在的 ID 一样返回 404。读路由为 Viewer+；写路由为“KB 创建者 OR Admin+”。下文的「KB read」/「KB write」是沿用的速记，都指路由挂了 KB 访问守卫（`KBAccess`，知识库须属于本工作区）；读写的区别只在角色与所有权守卫上。API key：读需 `retrieve`，内容写需 `ingest`，KB 生命周期需 `manage_kbs`（均可被 full-access 覆盖），并受 KB 白名单约束。

分块、标签与分块预览接口（`/chunks`、`/knowledge-bases/:id/tags`、`/chunker/preview`）在[分块与标签](./02-api-chunks.md)；知识库活动流 `GET /knowledge-bases/:id/activity` 在[租户与成员](./02-api-tenant.md)。

## 知识库（/api/v1/knowledge-bases）

### POST /api/v1/knowledge-bases

用途：创建知识库。权限：Contributor+；API key `manage_kbs`/full。Handler: `internal/handler/knowledgebase.go`

请求体（`types.KnowledgeBase`）：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 名称 |
| `description` | string | 否 | 描述 |
| `type` | string | 否 | `document`（默认）/`faq`/`wiki` |
| `embedding_model_id` | string | 否 | Embedding 模型 ID |
| `summary_model_id` | string | 否 | 摘要模型 ID |
| `chunking_config` | object | 否 | 分块配置（chunk_size/overlap/separators/strategy…） |
| `image_processing_config` / `vlm_config` / `asr_config` | object | 否 | 图像处理、VLM、语音识别配置 |
| `storage_backend_id` | string | 否 | 新文件写入的存储后端；缺省绑定工作区默认。响应里另有 `storage_backend`（`{id,name,provider,source,is_builtin}`） |
| `vector_store_id` | string | 否 | 检索引擎实例绑定，仅创建时可设（非法返回 code 2200，不可用返回 2201） |
| `faq_config` / `wiki_config` / `extract_config` / `indexing_strategy` | object | 否 | 类型相关配置 |
| `question_generation_config` / `auto_tag_config` | object | 否 | 问题生成、自动打标配置 |
| `review_interval_days` | int | 否 | 复核周期（天），0 表示不复核（默认），最大 3650；见[知识健康](../03-features/22-knowledge-health.md) |
| `is_temporary` | bool | 否 | 临时知识库（默认 false）；临时库不会出现在移动目标里 |

`auto_tag_config`（文档自动打标）：文档解析完成后异步调用聊天模型，从知识库**已有**标签里挑选匹配项并增量关联到文档；不会新建标签，也不会删除人工打的标签。

| 字段 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `enabled` | bool | false | 是否启用 |
| `model_id` | string | 空 | 聊天模型 ID；为空时使用 KB 的 `summary_model_id` |
| `max_tags` | int | 3 | 单个文档最多关联的标签数，范围 1-10 |
| `skip_if_tagged` | bool | true | 文档已有标签时跳过，不调用模型；设为 false 则在已有标签上追加 |

只对启用后新解析或重新解析的文档生效；模型调用失败不阻塞解析完成。候选标签最多取前 500 个。

响应：201 `{"success":true,"data":{KnowledgeBase}}`。创建、详情、列表、更新与置顶返回的 KB 对象都会附带解析后的向量存储元数据：

| 字段 | 说明 |
| --- | --- |
| `vector_store_name` | 存储展示名；未绑定时为 `System default` |
| `vector_store_source` | `env`（环境变量虚拟存储）/ `user`（数据库中创建的存储）/ `unavailable`（绑定的存储已无法解析） |
| `vector_store_engine_type` | 引擎类型 |
| `vector_store_status` | `available` / `unavailable`；`unavailable` 表示绑定的存储已被删除或未注册 |

```bash
curl -X POST $BASE/api/v1/knowledge-bases -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"产品文档","type":"document"}'
```

### GET /api/v1/knowledge-bases

用途：当前工作区的知识库列表（不分页）。权限：Viewer+；API key `retrieve`/full；KB 白名单受限的 key 只看到白名单内的 KB。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `creator` | string | 否 | `mine`（我创建的）/ `others`（同工作区其他成员创建的）；不传返回全部 |

响应：200 `{"success":true,"data":[KnowledgeBase]}`，每项附 `knowledge_count`、`chunk_count`、`processing_count`、`creator_name`、`is_pinned`/`pinned_at` 等统计与状态字段。

```bash
curl $BASE/api/v1/knowledge-bases -H "X-API-Key: $API_KEY"
```

### GET /api/v1/knowledge-bases/:id

用途：知识库详情。权限：Viewer+；别的工作区的知识库返回 404。

响应：200 `{"success":true,"data":{KnowledgeBase}}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id

用途：更新知识库。权限：创建者 OR Admin+，KB write；API key `manage_kbs`/full。`vector_store_id` 创建后不可修改，更新接口不接收该字段。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是（`binding:"required"`） | 名称 |
| `description` | string | 否 | 描述 |
| `config` | object | 否 | 配置更新（`types.KnowledgeBaseConfig`）：`chunking_config`、`image_processing_config`、`faq_config`、`wiki_config`、`auto_tag_config`、`indexing_strategy`、`review_interval_days`；`indexing_strategy` 与 `review_interval_days` 不传表示不改 |

响应：200 `{"success":true,"data":{KnowledgeBase}}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"产品文档 v2"}'
```

### POST /api/v1/knowledge-bases/:id/rebuild-index

用途：重建索引——逐篇重新解析 KB 内的全部文档，使修改后的 `indexing_strategy`（向量/关键词/Wiki/图谱）作用于已有文档。每篇文档按单篇 `reparse` 的规则处理，沿用其上传时保存的解析覆盖；草稿与删除中的文档跳过。异步执行：请求只统计文档数并排入 maintenance 队列，由后台任务分页逐篇提交解析，大库不会在请求里加载全部文档。权限：创建者 OR Admin+；API key `ingest`/full。无请求体。

同一 KB 已有重建在排队或进行中时返回 409，不会重复解析；FAQ 知识库没有可重新解析的文档，返回 400。没有可处理的文档时不排任务，返回 `document_count: 0`。

响应：200 `{"success":true,"data":{"task_id":"…","document_count":N}}`（`document_count` 为 0 时无 `task_id`）

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/rebuild-index -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/knowledge-bases/:id

用途：删除知识库，级联清理其下全部知识与分块。权限：创建者 OR Admin+；API key `manage_kbs`/full。

响应：200 `{"success":true,"message":"Knowledge base deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/knowledge-bases/kb-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id/pin

用途：置顶/取消置顶（按用户维度存储），每次调用翻转当前 `is_pinned`，置顶时写入 `pinned_at`。权限：Viewer+，KB read。无请求体。

响应：200 `{"success":true,"data":{KnowledgeBase(is_pinned 已切换)}}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/pin -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledge-bases/:id/hybrid-search

用途：KB 内混合检索（向量+关键词）。权限：Viewer+，KB read；API key `retrieve`/full。

请求体（`types.SearchParams`）：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `query_text` | string | 条件必填 | 查询文本（除非提供 `query_embedding`） |
| `query_embedding` | []float32 | 否 | 预计算向量 |
| `vector_threshold` / `keyword_threshold` | float64 | 否 | 匹配阈值 |
| `match_count` | int | 否 | 返回条数上限 |
| `disable_keywords_match` / `disable_vector_match` | bool | 否 | 关闭某一路召回 |
| `knowledge_ids` | []string | 否 | 限定知识条目 |
| `tag_ids` | []string | 否 | 标签过滤（OR） |
| `only_recommended` | bool | 否 | FAQ 仅推荐条目 |
| `knowledge_base_ids` | []string | 否 | 一次检索多个共用同一 Embedding 模型的 KB（覆盖路径中的 `:id`） |
| `skip_context_enrichment` | bool | 否 | 跳过父块/上下文补齐 |

响应：200 `{"success":true,"data":[SearchResult]}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/hybrid-search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"query_text":"退款流程","match_count":5}'
```

### POST /api/v1/knowledge-bases/copy

用途：拷贝整个知识库（配置 + 全部知识内容，异步任务）。任务入 asynq `low` 队列，最多重试 3 次、超时 2 小时，立即返回 `task_id` 供轮询。权限：Contributor+；API key `manage_kbs`/full（源/目标 KB 白名单在 handler 校验）。

源 KB 与目标 KB 都必须属于调用者所在工作区；属于其他工作区与不存在一样返回 404（`Source knowledge base not found` / `Target knowledge base not found`），和其他知识库路由的规则一致。指定 `target_id` 时会同步预检，失败直接返回 400、不入队：

- 两边的 Embedding 模型不同：`source and target knowledge bases use different embedding models; ...`
- 两边绑定的向量存储不同：`source and target knowledge bases are bound to different vector stores; ...`
- 两边的存储后端实例不同：`source and target knowledge bases use different storage instances; ...`

`target_id` 为空时新建目标库，并沿用源库的 `vector_store_id` 与 `embedding_model_id`，因此不会触发预检。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `source_id` | string | 是（`binding:"required"`） | 源 KB |
| `target_id` | string | 否 | 目标 KB（为空则自动创建） |
| `task_id` | string | 否 | 自定义任务 ID；不传由服务端按工作区、源 ID 与时间戳生成 |

响应：200 `{"success":true,"data":{"task_id","source_id","target_id","message"}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/copy -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"source_id":"kb-1"}'
```

### POST /api/v1/knowledge-bases/:id/duplicate

用途：同步创建 KB 副本，只复制设置（分块、模型、索引策略、FAQ/Wiki 配置等），不复制知识条目、分块、FAQ 条目、Wiki 页面、索引、数据源绑定与置顶状态。权限：Contributor+，源 KB read；API key `manage_kbs`/full。源 KB 必须属于调用者所在工作区；属于其他工作区与不存在一样返回 404 `Source knowledge base not found`。无请求体。

与 `/copy` 的区别：`/duplicate` 同步、只有设置、总是新建；`/copy` 异步、带全部内容、可写入已有目标库。

新副本的名称是源名称加本地化后缀（按 `Accept-Language` 或 `YUHENG_LANGUAGE`）：中文 ` 副本`、英文 ` Copy`、韩文 ` 사본`、俄文 ` копия`；同名已存在时继续递增编号。

响应：201 `{"success":true,"data":{"source_id","target_id","message","knowledge_base":{...}}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/duplicate -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/copy/progress/:task_id

用途：查询拷贝进度（任务按工作区隔离）。权限：Viewer+；API key `retrieve`/`manage_kbs`/full。

响应：200 `{"success":true,"data":{task_id,source_id,target_id,status,progress,total,processed,message,error,created_at,updated_at}}`。`status` 为 `pending`/`processing`/`completed`/`failed`，`progress` 为 0-100，`total`/`processed` 是知识条数，时间为 Unix 秒。

```bash
curl $BASE/api/v1/knowledge-bases/copy/progress/task-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/move-targets

用途：列出本工作区内可作为移动目标的 KB：与源 KB `type` 相同、`embedding_model_id` 相同、非临时库，且不含源 KB 自身。权限：Viewer+，KB read。

响应：200 `{"success":true,"data":[KnowledgeBase]}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/move-targets -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/files

用途：KB 范围文件代理，渲染知识库内容（分块、Wiki 页面）里嵌入的图片；只读本工作区 `exports/` 区域的资源，不给原始上传文件。权限：Viewer+，KB 须属于本工作区；KB 受限 key 拒绝，全工作区 `retrieve`/full key 放行。注册于 `serveKBScopedFiles`（`internal/router/files.go`），与其它文件访问方式的对比见[文件服务](./02-api-files.md)。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file_path` | string | 是 | `resource://<handle>` 引用；须属于本工作区、位于其 `exports/` 区域 |

响应：200 文件流（`Content-Type` 按资源记录推断；`Cache-Control: private`）。

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/files?file_path=resource://AbCdEfGhIjKlMnOpQrStUv" \
  -H "Authorization: Bearer $TOKEN" -o chart.png
```

## 知识（KB 内容，/api/v1/knowledge-bases/:id/knowledge 与 /api/v1/knowledge）

路径里 `/knowledge-bases/:id/...` 的 `:id` 是知识库 ID，`/knowledge/:id` 的 `:id` 是知识 ID。

`parse_status` 取值：`pending`（排队）、`processing`（DocReader / 分块 / 向量化）、`finalizing`（主解析已完成、已可检索，摘要 / 问题生成 / 图谱抽取等子任务仍在执行，`pending_subtasks_count` 为剩余数）、`completed`（全部子任务到达终态）、`failed`、`cancelled`（用户取消，已写入的分块与索引保留，可 `reparse`）、`deleting`（删除中）。

### POST /api/v1/knowledge-bases/:id/knowledge/file

用途：上传文件创建知识。权限：KB 创建者 OR Admin+，KB write；API key `ingest`/full。Handler: `internal/handler/knowledge.go`

multipart/form-data 字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | file | 是 | 上传文件 |
| `fileName` | string | 否 | 覆盖显示名；整目录上传时用它保留相对路径（如 `docs/intro.md`） |
| `metadata` | JSON 字符串 | 否 | 反序列化为 `map[string]string`，格式错误返回 400 |
| `enable_multimodel` | bool 字符串 | 否 | `true`/`false`，多模态处理开关 |
| `tag_ids` | string | 否 | 逗号分隔标签 ID |
| `channel` | string | 否 | 摄取渠道，默认 `web` |
| `process_config` | JSON 字符串 | 否 | 本次解析配置覆盖（`KnowledgeProcessOverrides`）：`parser_engine_rules`、`chunking_config`、`enable_multimodel`、`vlm_config`、`asr_config`、`question_generation_config`、`graph_enabled`、`extract_config`；与表单 `enable_multimodel` 同时给出时以 `process_config` 为准 |

文件大小上限取系统设置 `file.max_size_mb`（视频文件取 `file.video_max_size_mb`，见 `GET /system/upload-limits`），超限返回 400（`文件大小不能超过NMB`）。用 curl `-F` 上传时不要再手动设置 `Content-Type: application/json`。

响应：200 `{"success":true,"data":{Knowledge}}`，此时解析任务已入队；重复文件返回 409 且 `data` 为已存在的 Knowledge。

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/knowledge/file \
  -H "X-API-Key: $API_KEY" -F 'file=@./manual.pdf' -F 'enable_multimodel=true'
```

### POST /api/v1/knowledge-bases/:id/knowledge/url

用途：从 URL 抓取创建知识。权限/API key 同上。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `url` | string | 是（`binding:"required"`） | 抓取地址，经过 SSRF 校验，不能指向内网或回环地址 |
| `file_name` / `file_type` / `title` | string | 否 | 覆盖信息；给出 `file_name` 或 `file_type` 会强制走文件下载模式 |
| `enable_multimodel` | *bool | 否 | 多模态开关 |
| `tag_ids` | []string | 否 | 标签 |
| `channel` | string | 否 | 渠道 |
| `process_config` | object | 否 | 解析覆盖 |

URL 路径带受支持的文件扩展名，或显式给出 `file_name`/`file_type` 时，按「文件下载模式」拉取远端文件；否则按「网页抓取模式」处理。

响应：201 `{"success":true,"data":{Knowledge}}`；重复 URL 返回 409。

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/knowledge/url -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"url":"https://example.com/doc"}'
```

### POST /api/v1/knowledge-bases/:id/knowledge/manual

用途：创建手工（Markdown）知识。权限/API key 同上。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `title` | string | 否 | 标题；为空时生成 `Knowledge-<时间戳>` |
| `content` | string | 是 | Markdown 内容，清洗后不能为空，最多 200000 个字符 |
| `status` | string | 否 | `draft`（默认，不触发解析）/ `publish`（入队解析）；其它值返回 1010 |
| `tag_ids` | []string | 否 | 标签 |
| `channel` | string | 否 | 渠道 |
| `process_config` | object | 否 | 解析覆盖 |

响应：200 `{"success":true,"data":{Knowledge}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/knowledge/manual -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"title":"FAQ 汇总","content":"# 内容","status":"publish"}'
```

### GET /api/v1/knowledge-bases/:id/knowledge

用途：KB 下知识列表。权限：Viewer+，KB read；API key `retrieve`/full。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `page` / `page_size` | int | 否 | 分页（默认 1/20，`page_size` 最大 1000） |
| `tag_ids` | string | 否 | 逗号分隔标签（OR） |
| `keyword` | string | 否 | 关键字 |
| `file_type` | string | 否 | 文件类型过滤 |
| `parse_status` | string | 否 | `pending/processing/completed/failed` |
| `source` | string | 否 | `manual`/`url` 按知识类型过滤，其它值按摄取渠道 `channel` 过滤 |
| `start_time` / `end_time` | string | 否 | 按 `updated_at` 过滤；接受 RFC3339、`YYYY-MM-DD HH:MM:SS`、`YYYY-MM-DD`（后两种按服务器本地时区），格式错误返回 400 |
| `folder_path` | string | 否 | 按文件夹过滤；参数出现即生效，空字符串表示根目录 |
| `folder_recursive` | bool | 否 | 与 `folder_path` 同用，`true` 时包含子目录 |

响应：200 `{"success":true,"data":[Knowledge],"total","page","page_size"}`

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/knowledge?page=1&parse_status=completed" -H "X-API-Key: $API_KEY"
```

### GET /api/v1/knowledge-bases/:id/knowledge/folders

用途：获取知识库的文件夹目录树。整目录上传时目录结构会被保留（migration `000079` 起存在 `knowledges.folder_path` 列，早期把路径塞在 `file_name` 里的数据已回填）。权限：Viewer+ + KBAccess。

响应：200 `{"success":true,"data":[{KnowledgeFolderNode}]}`，节点字段：`path`、`name`（最后一段）、`document_count`（直接位于该目录的文档数）、`total_count`（含子目录）、`children`（按名称排序）。

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/knowledge/folders -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id/knowledge/folders

用途：重命名或移动文件夹，连同其所有子目录一起改路径。目标路径已存在时两个文件夹合并；不允许移动到自己的子目录下。权限：KB owner 或 Admin+ + KBAccess。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `from` | string | 是 | 原路径 |
| `to` | string | 是 | 新路径 |

响应：200 `{"success":true,"data":{"moved_count":N,"folder_path":"<规范化后的新路径>"}}`；`moved_count` 为 0 表示源文件夹不存在或是空操作。

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/knowledge/folders -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"from":"设计文档/旧版","to":"归档/设计文档"}'
```

### DELETE /api/v1/knowledge-bases/:id/knowledge

用途：清空 KB 全部内容（破坏性）。权限：Admin+，KB write；API key 仅 full-access。

只删除知识，知识库本身保留；非属主工作区返回 403 `Only knowledge base owner can clear contents`。

响应：200 `{"success":true,"message":"Knowledge base contents clear task submitted","data":{"deleted_count":N}}`；知识库本来就空时返回 `{"success":true,"message":"Knowledge base is already empty","data":{"deleted_count":0}}`。

```bash
curl -X DELETE $BASE/api/v1/knowledge-bases/kb-1/knowledge -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/batch

用途：按 ID 批量获取知识（跨 KB，handler 自行校验访问）。权限：Viewer+；API key `retrieve`/full。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `ids` | []string | 是 | 知识 ID（可重复传参或逗号分隔） |
| `kb_id` | string | 否 | 限定 KB（校验访问权限） |

响应：200 `{"success":true,"data":[Knowledge]}`

```bash
curl "$BASE/api/v1/knowledge/batch?ids=k-1&ids=k-2" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id

用途：知识详情。权限：Viewer+，父 KB read。

响应：200 `{"success":true,"data":{Knowledge}}`

```bash
curl $BASE/api/v1/knowledge/k-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id/stages 与 GET /api/v1/knowledge/:id/spans

用途：解析阶段/trace（两条路径同一 handler `GetKnowledgeSpans`）。权限：Viewer+，父 KB read。查询参数：`attempt`（int，0=最新一次）。

响应：200 `{"success":true,"data":{"knowledge_id","attempt","latest_attempt","parse_status","current_stage","trace":{...},"last_error":{...}}}`

```bash
curl $BASE/api/v1/knowledge/k-1/spans -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/knowledge/:id

用途：删除知识（异步）。权限：KB 创建者 OR Admin+，KB write；API key `ingest`/full。

响应：200 `{"success":true,"message":"Delete task submitted","data":{"task_id"}}`

```bash
curl -X DELETE $BASE/api/v1/knowledge/k-1 -H "X-API-Key: $API_KEY"
```

### PUT /api/v1/knowledge/:id

用途：部分更新知识元信息。权限同上。请求体（`UpdateKnowledgeRequest`）：`title`、`description`、`custom_metadata`（均可选，未传字段保持不变；显式传 `"description":""` 可清空摘要）。标签用 `PUT /knowledge/tags` 修改。

`custom_metadata` 是用户自填的描述性元数据（与系统内部使用的 `metadata` 分开存放，migration `000078`），校验规则见 `internal/application/service/knowledge.go`：

| 约束 | 值 |
| --- | --- |
| 字段数 | ≤ 20 |
| 键长度 | 1-64 字符，不能为空白 |
| 值类型 | string / number / boolean / null |
| 值长度 | ≤ 1000 字符 |

整体覆盖式更新（传入的对象替换原有对象）。元数据发生变化且该文档已有摘要时，会自动入队一次摘要刷新（`summary_status` 转为 `pending`）。元数据文本会参与摘要生成与文档级模型上下文（`Knowledge.CustomMetadataText()`）。

响应：200 `{"success":true,"message":"Knowledge updated successfully","data":{Knowledge}}`

```bash
curl -X PUT $BASE/api/v1/knowledge/k-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"新标题","custom_metadata":{"部门":"研发中心","密级":"内部","版本":3}}'
```

### POST /api/v1/knowledge/:id/regenerate-summary

用途：在分块内容或自定义元数据被编辑后，重新生成该文档的摘要。权限：KB owner 或 Admin+，且对父 KB 有 write 权限。

行为分两种：文档此前没有摘要（`summary_status` 为空或 `none`）时同步触发一次生成；已有摘要时改为入队刷新任务，`summary_status` 转为 `pending`，由 `knowledge_summary_refresh.go` 异步执行。

响应：200 `{"success":true,"data":{Knowledge}}`

```bash
curl -X POST $BASE/api/v1/knowledge/k-1/regenerate-summary -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge/manual/:id

用途：更新手工知识内容（`ManualKnowledgePayload` 子集：`title/content/status/...`）。权限同上。

响应：200 `{"success":true,"data":{Knowledge}}`

```bash
curl -X PUT $BASE/api/v1/knowledge/manual/k-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"content":"# 更新内容","status":"publish"}'
```

### POST /api/v1/knowledge/:id/reparse

用途：重新解析知识。权限同上。请求体（可选）：`{"process_config":{...}}`。

响应：200 `{"success":true,"message":"Reparse task submitted","data":{Knowledge}}`

```bash
curl -X POST $BASE/api/v1/knowledge/k-1/reparse -H "X-API-Key: $API_KEY"
```

### POST /api/v1/knowledge/:id/cancel-parse

用途：取消解析。权限同上。无请求体。

- 可取消的状态：`pending`、`processing`、`finalizing`；在 `finalizing` 取消可以及时停掉摘要、问题生成与图谱抽取的模型消耗。
- `completed`/`failed` 返回 400（`解析已结束，无法取消`），`deleting` 返回 400（`知识正在删除中，无法取消解析`）；对已 `cancelled` 的记录重复调用直接返回当前状态（幂等）。
- 成功后 `parse_status=cancelled`、`error_message` 为「用户已取消解析」、`pending_subtasks_count` 清零；已写入的分块与索引保留，下游排队任务尽力撤销，运行中的 worker 在下一个检查点退出。

响应：200 `{"success":true,"message":"Knowledge parse cancelled","data":{Knowledge}}`

```bash
curl -X POST $BASE/api/v1/knowledge/k-1/cancel-parse -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id/download

用途：下载原始源文件（比预览更严格：Contributor+；Viewer 不可下载源文件）。API key `retrieve`/full。

响应：200 二进制流（`Content-Type: application/octet-stream`，`Content-Disposition: attachment; filename="..."`）。

```bash
curl -OJ $BASE/api/v1/knowledge/k-1/download -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id/preview

用途：预览解析后的文件内容。权限：Viewer+，KB read。

响应：200 原始文件流：`Content-Type` 按扩展名推断，可安全内联的类型返回 `Content-Disposition: inline`，其余降级为 `attachment`；带 `X-Content-Type-Options: nosniff` 与 `Cache-Control: private, max-age=3600`。

```bash
curl $BASE/api/v1/knowledge/k-1/preview -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge/image/:id/:chunk_id

用途：更新某分块的图片信息（caption/OCR 等）。权限：KB 创建者 OR Admin+，KB write。路径参数：`id` 知识 ID、`chunk_id` 分块 ID。请求体：`{"image_info":"<图片信息数组的 JSON 字符串>"}`。

响应：200 `{"success":true,"message":"Knowledge chunk image updated successfully"}`

```bash
curl -X PUT $BASE/api/v1/knowledge/image/k-1/c-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"image_info":"[{\"url\":\"...\",\"caption\":\"架构图\"}]"}'
```

### GET /api/v1/knowledge/search

用途：按文件名跨 KB 搜索知识（会话里 @文件 的选择器），范围是本工作区的 KB；KB 白名单受限的 API key 只搜白名单内的 KB。权限：Viewer+；API key `retrieve`/full。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `keyword`（或 `query`） | string | 条件必填 | 关键字；为空时必须 `recent=true`，否则 400 |
| `recent` | bool | 否 | 关键字为空时返回最近文件 |
| `file_types` | string | 否 | 逗号分隔的扩展名，如 `csv,xlsx` |
| `offset` / `limit` | int | 否 | 偏移分页，`limit` 默认 20、最大 100 |

响应：200 `{"success":true,"data":[Knowledge],"has_more":bool,"total":N}`

```bash
curl "$BASE/api/v1/knowledge/search?keyword=报告&limit=10" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/move/progress/:task_id

用途：查询移动任务进度。权限：Viewer+；API key `retrieve`/full。

响应：200 `{"success":true,"data":{task_id,source_kb_id,target_kb_id,status,progress,total,processed,failed,message,error,created_at,updated_at}}`，`status` 取值同拷贝进度，时间为 Unix 秒。

```bash
curl $BASE/api/v1/knowledge/move/progress/task-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge/tags

用途：批量更新知识标签。权限：Contributor+；API key `ingest`/full（KB 白名单在 handler 校验）。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `updates` | map[string][]string | 是（`binding:"required,min=1"`） | knowledge_id → tag_ids |
| `kb_id` | string | 否 | 限定 KB，按该 KB 校验写权限；不传时取 `updates` 中第一个知识所属的 KB 鉴权 |

响应：200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/knowledge/tags -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"updates":{"k-1":["t-1"]},"kb_id":"kb-1"}'
```

### POST /api/v1/knowledge/batch-reparse

用途：批量重解析。权限：Contributor+；API key `ingest`/full。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `kb_id` | string | 是（`binding:"required"`） | KB ID |
| `ids` | []string | 是（`binding:"required"`） | 知识 ID 列表（≤200） |
| `process_config` | object | 否 | 本批共用的解析覆盖；不传沿用各知识原配置 |

任一 ID 不存在或不属于 `kb_id` 时返回 400，整批拒绝。

响应：200 `{"success":true,"message":"Batch reparse task submitted","data":{"task_id"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/batch-reparse -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"kb_id":"kb-1","ids":["k-1","k-2"]}'
```

### POST /api/v1/knowledge/batch-delete

用途：批量删除（≤200 条）。权限：Contributor+；API key `ingest`/full。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `kb_id` | string | 是（`binding:"required"`） | KB ID |
| `ids` | []string | 是（`binding:"required"`） | 知识 ID 列表（≤200） |

任一 ID 不存在或不属于 `kb_id` 时返回 400（`One or more knowledge entries not found`），整批拒绝。

响应：200 `{"success":true,"message":"Batch delete task submitted","data":{"task_id","deleted_count"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/batch-delete -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"kb_id":"kb-1","ids":["k-1"]}'
```

### POST /api/v1/knowledge/folder

用途：把若干文档归类到指定文件夹（只改归类，不动知识库归属，也不重新解析）。权限：Contributor+ / API key `ingest`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `kb_id` | string | 是 | 知识库 ID |
| `knowledge_ids` | []string | 是 | 待移动的文档（≤200） |
| `folder_path` | string | 否 | 目标文件夹，不存在时自动创建；空字符串表示移回知识库根目录 |

响应：200 `{"success":true,"data":{"moved_count":N,"folder_path":"<规范化后的路径>"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/folder -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"kb_id":"kb-1","knowledge_ids":["k-1","k-2"],"folder_path":"设计文档"}'
```

### POST /api/v1/knowledge/move

用途：跨 KB 移动知识（异步）。权限：Contributor+；API key `ingest`/full（源+目标 KB 均需在白名单）。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `knowledge_ids` | []string | 是（`binding:"required,min=1"`） | 待移动知识 |
| `source_kb_id` | string | 是（`binding:"required"`） | 源 KB |
| `target_kb_id` | string | 是（`binding:"required"`） | 目标 KB |
| `mode` | string | 是（`binding:"required,oneof=reuse_vectors reparse"`） | `reuse_vectors` 直接复用向量数据；`reparse` 在目标库重新解析 |

约束（违反返回 400）：源 KB 与目标 KB 不能相同，且类型相同、Embedding 模型相同；只能移动源 KB 中 `parse_status=completed` 的知识；`reuse_vectors` 要求两边使用同一个向量存储，否则改用 `reparse`。源或目标 KB 不存在 404，无权访问 403。

响应：200 `{"success":true,"data":{"task_id","source_kb_id","target_kb_id","knowledge_count","message"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/move -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"knowledge_ids":["k-1"],"source_kb_id":"kb-1","target_kb_id":"kb-2","mode":"reuse_vectors"}'
```

## 知识健康

检测知识库里重复、内容有出入、久未复核以及回答被反馈有误的文档，并把问题派给负责人处理。检测机制、问题类型（`duplicate` / `divergent` / `stale` / `disputed`）、派发规则与记录结构见[知识健康](../03-features/22-knowledge-health.md)，这里只列接口要点。复核周期由知识库的 `review_interval_days` 决定（见上文创建与更新知识库）。回答反馈接口（`/sessions/:id/feedback`）在[会话与聊天](./02-api-chat.md)。

问题的状态为 `open`（未处理）、`dismissed`（已忽略，证据变化时重新打开）、`resolved`（检测不再报告或已处理）。

### GET /api/v1/knowledge-bases/:id/findings

用途：问题列表。权限：Viewer+，KB read；API key `retrieve`/full。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `status` | string | 否 | `open`（默认）/ `dismissed` / `resolved` / `all` |
| `type` | string | 否 | 问题类型，如 `duplicate` |
| `knowledge_id` | string | 否 | 只看涉及该文档的问题 |
| `assignee` | string | 否 | `me`：只看派给我的 |
| `page` / `page_size` | int | 否 | 默认 1/20，`page_size` 最大 100 |

响应：200 `{"success":true,"data":{"items":[Finding],"total","page","page_size"}}`

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/findings?status=open" -H "X-API-Key: $API_KEY"
```

### GET /api/v1/knowledge-bases/:id/findings/summary

用途：健康概览。权限同上。

响应：200 `{"success":true,"data":{"open_total","open_by_type":{...},"last_scan_at","enabled","supported"}}`。`enabled` 表示本部署是否开启检测（`YUHENG_FINDINGS_ENABLED`），`supported` 表示该知识库的检索引擎能否做内容比对。

### PATCH /api/v1/knowledge-bases/:id/findings/:finding_id

用途：忽略或重新打开问题，操作记入知识库活动流。权限：KB 创建者 OR Admin+，KB write；API key `ingest`/full。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `status` | string | 是 | `dismissed` 或 `open` |
| `reason` | string | 忽略时必填 | `distinct_scope`（适用范围不同）/ `intentional`（有意保留） |

响应：200 `{"success":true,"data":{Finding}}`

```bash
curl -X PATCH $BASE/api/v1/knowledge-bases/kb-1/findings/f-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"status":"dismissed","reason":"intentional"}'
```

### PUT /api/v1/knowledge-bases/:id/findings/:finding_id/assignee

用途：手动指派处理人（之后自动检测不再改派）；`assignee_id` 为空字符串表示交还自动派发并立即重新派发。处理人须能编辑该知识库。权限同上。请求体：`{"assignee_id":"<user_id>"}`。

响应：200 `{"success":true,"data":{Finding}}`

### POST /api/v1/knowledge-bases/:id/findings/:finding_id/supersede

用途：处理重复或内容有出入的问题：保留一份，另一份退出知识库。上传的文档被删除；在线文档页面被排除出知识库并标记为已被取代；数据源同步的文档无法在这里移除，返回 409。权限同上。请求体：`{"keep_knowledge_id":"<问题涉及的两份文档之一>"}`。

响应：200 `{"success":true,"data":{"retired_knowledge_id","how"}}`，`how` 为 `deleted` 或 `excluded`。

### POST /api/v1/knowledge-bases/:id/findings/scan

用途：为知识库中所有已完成索引的文档安排一次重新检测（分批执行，已在排队的不重复安排）。权限：KB 创建者 OR Admin+；API key `manage_kbs`/full。无请求体。

响应：200 `{"success":true,"data":{"queued":N}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/findings/scan -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/findings/assigned 与 GET /api/v1/findings/assigned/count

用途：当前工作区里派给我的问题（跨知识库，每项附 `knowledge_base_name`）及未处理数量。权限：Viewer+。API key 可调用（`retrieve`/full），但 key 不是具体用户，结果为空。

`/findings/assigned` 查询参数：`status`（同列表，默认 `open`）、`page`、`page_size`；响应 `{"success":true,"data":{"items","total","page","page_size"}}`。`/count` 响应 `{"success":true,"data":{"open_total":N}}`。

### GET /api/v1/knowledge/:id/stewardship

用途：文档的负责人与复核状态。权限：Viewer+，父 KB read；API key `retrieve`/full。

响应：200 `{"success":true,"data":{"knowledge_id","origin","owner","owner_editable","reviewed_at","reviewed_by","review_interval_days","review_due_at","overdue"}}`

### PUT /api/v1/knowledge/:id/owner

用途：转交文档负责人，新负责人须是能编辑该知识库的在职成员；`owner_id` 为空字符串表示不设负责人。在线文档页面镜像的负责人要在页面上修改（这里返回 409）。操作记入知识库活动流。权限：KB 创建者 OR Admin+，KB write；API key `ingest`/full。请求体：`{"owner_id":"<user_id>"}`。

响应：200，`data` 同 stewardship。

### POST /api/v1/knowledge/:id/review

用途：确认文档仍然有效：重新开始复核计时，并关闭该文档的“需要复核”和“回答被反馈有误”问题。需要登录用户，API key 调用返回 403。权限：KB 创建者 OR Admin+，KB write。无请求体。

响应：200，`data` 同 stewardship。

```bash
curl -X POST $BASE/api/v1/knowledge/k-1/review -H "Authorization: Bearer $TOKEN"
```
