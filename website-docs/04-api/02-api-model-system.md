# API 参考：模型与初始化

路由注册：`internal/router/routes_infra.go` 的 `RegisterModelRoutes`、`RegisterInitializationRoutes`、`RegisterEvaluationRoutes`。Handler：`internal/handler/model.go`、`internal/handler/model_credentials.go`、`internal/handler/initialization.go`、`internal/handler/evaluation.go`。

系统信息与系统管理（`/system`、`/system/admin`）接口见[系统与平台管理](./02-api-system.md)。

## 模型（/api/v1/models）

API key：`manage_models` 或 full-access。读接口 Viewer+；所有写操作、调试与连通性检测挂 `PlatformManaged` 守卫：系统设置 `governance.centralized_infra` 关闭时为 Admin+，开启后只允许系统管理员（下文写作“权限：平台管理”）。模型类型取值：`KnowledgeQA`（对话）、`Embedding`、`Rerank`、`VLLM`（视觉）、`ASR`（语音识别）。

### GET /api/v1/models/providers

用途：模型厂商列表。权限：Viewer+。查询参数：`model_type`（可选：`chat/embedding/rerank/vllm/asr`，分别映射到上述模型类型；省略时返回全部厂商）。Handler: `internal/handler/model.go`

响应：200 `{"success":true,"data":[{value,label,description,defaultUrls,modelTypes}]}`。`value` 是厂商标识，填到创建模型时的 `parameters.provider`，用来选择对应的 API 适配器；`defaultUrls` 按模型类型（`chat`、`embedding`、`rerank` 等）给出该厂商的默认接口地址；`modelTypes` 是该厂商支持的模型类型（前端别名）。这是系统级元数据，与空间无关。

代码中注册的厂商标识（`internal/models/provider/provider.go`）：`generic`（自定义 OpenAI 兼容接口）、`openai`、`azure_openai`、`anthropic`、`aliyun`、`zhipu`、`volcengine`、`hunyuan`、`lkeap`、`deepseek`、`minimax`、`mimo`、`moonshot`、`qianfan`、`qiniu`、`longcat`、`siliconflow`、`jina`、`openrouter`、`requesty`、`gemini`、`modelscope`、`gpustack`、`nvidia`、`novita`。某个厂商支持哪些模型类型以接口返回为准。

```bash
curl "$BASE/api/v1/models/providers?model_type=chat" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/models

用途：创建模型。权限：平台管理。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是（`binding:"required"`） | 模型名 |
| `display_name` | string | 否 | 显示名 |
| `type` | string | 是（`binding:"required"`） | 模型类型（`KnowledgeQA/Embedding/Rerank/VLLM/ASR`） |
| `source` | string | 是（`binding:"required"`） | 来源：`local`（Ollama）、`remote`（OpenAI 兼容接口），或厂商标识如 `openai`、`aliyun`、`zhipu`、`deepseek`、`azure_openai` 等（`internal/types/model.go`） |
| `description` | string | 否 | 描述 |
| `parameters` | object | 是（`binding:"required"`） | 连接参数（base_url 等；密钥经 credentials 子资源管理） |
| `is_builtin` | bool | 否 | 设为平台共享的内置模型；仅系统管理员有效，其他人传入时按 false 处理 |

`parameters`（`types.ModelParameters`）字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `base_url` | string | 接口地址；非空时做 SSRF 校验，不通过返回 400。本地 Ollama 模型可留空 |
| `api_key` | string | API 密钥，AES-256 加密存储；创建时可以直接放在这里，之后的修改走 credentials 子资源 |
| `app_id` / `app_secret` | string | 应用级凭证，用于 app-id + 签名密钥类厂商；`app_secret` 加密存储 |
| `provider` | string | 厂商标识（见 `GET /models/providers` 的 `value`），用于选择 API 适配器 |
| `interface_type` | string | 接口风格；OpenAI 兼容接口留空 |
| `embedding_parameters` | object | Embedding 专用：`dimension`（向量维度）、`truncate_prompt_tokens`（0 表示不截断）、`supports_dimension_override` |
| `parameter_size` | string | 模型规模（如 `7B`），由后端写入，请求里无需提供 |
| `extra_config` | map[string]string | 厂商特定的额外配置 |
| `custom_headers` | map[string]string | 调用上游时附加的 HTTP 头；`Authorization`、`api-key`、`Content-Type`、`Accept` 等保留头会被忽略 |
| `supports_vision` | bool | 模型是否接受图像输入 |
| `max_concurrency` | int | 该模型后台任务（入库、增强）的并发上限，跨副本共享；0 表示用全局 `model.max_concurrency`，交互式调用不受限 |

火山引擎 Rerank 用 AK/SK 签名而不是方舟 API Key：`api_key` 填 Access Key ID，`app_secret` 填 Secret Access Key。

响应：201 `{"success":true,"data":{ModelResponse}}`（`id,tenant_id,name,display_name,type,source,description,parameters,is_default,is_builtin,status,credentials,created_at,updated_at`）。`status` 取值 `active`、`downloading`、`download_failed`。响应里永远没有 `api_key` 与 `app_secret`，只在 `credentials` 里给出 `{"api_key":{"configured":bool},"app_secret":{"configured":bool}}`。空间角色低于 Admin 的成员，以及既非 full-access 也没有 `manage_tenant_settings` 的 API key，看不到 `base_url`、`extra_config`、`custom_headers`；平台共享（内置）模型对非系统管理员还会隐去 `app_id` 且不返回 `credentials`，只保留描述能力的字段（`embedding_parameters`、`parameter_size`、`provider`、`interface_type`、`supports_vision`）。

```bash
curl -X POST $BASE/api/v1/models -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"gpt-4o-mini","type":"KnowledgeQA","source":"remote","parameters":{"base_url":"https://api.openai.com/v1"}}'

# Embedding 模型，指定厂商与维度
curl -X POST $BASE/api/v1/models -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"text-embedding-v3","type":"Embedding","source":"remote","parameters":{"base_url":"https://dashscope.aliyuncs.com/compatible-mode/v1","api_key":"sk-...","provider":"aliyun","embedding_parameters":{"dimension":1024,"truncate_prompt_tokens":0}}}'
```

### GET /api/v1/models

用途：模型列表。权限：Viewer+。

响应：200 `{"success":true,"data":[ModelResponse]}`

```bash
curl $BASE/api/v1/models -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/models/:id

用途：模型详情。权限：Viewer+。

响应：200 `{"success":true,"data":{ModelResponse}}`；模型不存在 404。

```bash
curl $BASE/api/v1/models/m-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/models/:id/debug

用途：调试已保存模型（发起真实上游调用，产生费用）。权限：平台管理。form-data 字段：`input`（≤64KB）、`options`（JSON 编码调试选项）、`documents`（JSON 数组，≤100 条）、`file`（可选）。

响应：200 `{"success":true,"data":{"ok",elapsed_ms,request,raw_response,observations,error}}`

```bash
curl -X POST $BASE/api/v1/models/m-1/debug -H "Authorization: Bearer $TOKEN" -F 'input=你好'
```

### PUT /api/v1/models/:id

用途：更新模型。权限：平台管理；内置模型由服务层额外限定系统管理员。请求体：`name`、`display_name`（指针）、`description`、`parameters`、`source`、`type`（均可选）。合并规则：

- `name` 为空串时保留原值；`display_name` 省略时保留原值；`description` 总是覆盖，传空串即清空。
- `parameters` 整体替换，但 `api_key` 与 `app_secret` 一律保留库中的值，请求里带了也会被忽略（服务端记一条弃用警告），改密钥走 `PUT /models/:id/credentials`；`parameter_size` 由后端维护；`interface_type`、`app_id` 为空、`extra_config` 省略时沿用旧值。
- 新的 `base_url` 同样做 SSRF 校验，不通过返回 400。

响应：200 `{"success":true,"data":{ModelResponse}}`；模型不存在 404。

```bash
curl -X PUT $BASE/api/v1/models/m-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"display_name":"GPT-4o mini"}'
```

### DELETE /api/v1/models/:id

用途：删除模型。权限：平台管理。

响应：200 `{"success":true,"message":"Model deleted"}`；模型不存在 404。

```bash
curl -X DELETE $BASE/api/v1/models/m-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/models/:id/credentials

用途：设置模型密钥（密钥不经主 PUT 传输）。权限：平台管理。Handler: `internal/handler/model_credentials.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `api_key` | *string | 否 | 新 API Key |
| `app_secret` | *string | 否 | 新 App Secret（两者均省略时仅返回状态） |

响应：200 `{"success":true,"data":{"fields":{"api_key":{"configured":bool},"app_secret":{"configured":bool}}}}`

```bash
curl -X PUT $BASE/api/v1/models/m-1/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"api_key":"sk-..."}'
```

### DELETE /api/v1/models/:id/credentials/:field

用途：删除某个密钥字段（`api_key` 或 `app_secret`）。权限：平台管理。

响应：204 No Content

```bash
curl -X DELETE $BASE/api/v1/models/m-1/credentials/api_key -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/models/:id/sharing

用途：设为平台共享（所有空间可见可用，凭证对非系统管理员隐藏）或取消共享。共享状态单独成一个子资源，避免普通编辑误改。权限：平台管理，服务层限定系统管理员。请求体：`{"shared":true}`（必填）。

响应：200 `{"success":true,"data":{ModelResponse}}`；取消共享时若仍有其他空间的知识库绑定该模型，返回 400；非系统管理员 403。

```bash
curl -X PUT $BASE/api/v1/models/m-1/sharing -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"shared":true}'
```

## 初始化（/api/v1/initialization）

Handler: `internal/handler/initialization.go`。KB 配置类：API key `manage_kbs`（写）/`retrieve`（读）；Ollama、模型检测与抽取类：`manage_models`（均可 full-access）。

### GET /api/v1/initialization/config/:kbId

用途：读取 KB 当前模型/解析配置。权限：Viewer+，KB read。

响应：200 `{"success":true,"data":{"hasFiles",llm,embedding,rerank,multimodal,documentSplitting,nodeExtract,questionGeneration}}`

```bash
curl $BASE/api/v1/initialization/config/kb-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/initialization/initialize/:kbId

用途：初始化 KB 的模型与解析配置（首次配置向导）。权限：KB 创建者 OR Admin+，KB write。

主要字段（`InitializationRequest`）：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `llm.source` / `llm.modelName` | string | 是 | LLM 来源与模型名 |
| `llm.baseUrl` / `llm.apiKey` | string | 否 | 连接参数 |
| `embedding.source` / `embedding.modelName` | string | 是 | Embedding 模型 |
| `embedding.baseUrl` / `embedding.apiKey` / `embedding.dimension` | — | 否 | 连接与维度 |
| `rerank.enabled` + `rerank.modelName/baseUrl/apiKey` | — | 否 | Rerank 配置 |
| `multimodal.enabled` + `multimodal.vlm.*` + `multimodal.storageType` | — | 否 | 多模态与图床；`storageType` 为空、`local` 或 `s3`，S3 连接参数取自服务端 `S3_*` 环境变量 |
| `documentSplitting.chunkSize` / `separators` | int / []string | 是 | 分块配置 |
| `documentSplitting.chunkOverlap` | int | 否 | 重叠 |
| `nodeExtract.*` | — | 否 | 图谱抽取（enabled/text/tags/nodes/relations） |
| `questionGeneration.*` | — | 否 | 问题生成（enabled/questionCount） |

响应：200 `{"success":true,"message":"知识库配置更新成功","data":{"models":[Model],"knowledge_base":{KnowledgeBase}}}`

```bash
curl -X POST $BASE/api/v1/initialization/initialize/kb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"llm":{"source":"remote","modelName":"gpt-4o-mini"},"embedding":{"source":"remote","modelName":"text-embedding-3-small"},"documentSplitting":{"chunkSize":512,"separators":["\n\n"]}}'
```

### PUT /api/v1/initialization/config/:kbId

用途：更新 KB 模型/分块配置（`KBModelConfigRequest`：`llmModelId` 必填，`embeddingModelId`、`vlm_config`、`asr_config`、`documentSplitting.*`、`multimodal.enabled`、`storageProvider`、`storageBackendId`、`nodeExtract.*`、`questionGeneration.*` 可选）。权限：KB 创建者 OR Admin+，KB write。

响应：200 `{"success":true,"message":"配置更新成功"}`

```bash
curl -X PUT $BASE/api/v1/initialization/config/kb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"llmModelId":"m-1","embeddingModelId":"m-2"}'
```

### GET /api/v1/initialization/ollama/status

用途：Ollama 可用性探测。权限：Viewer+。

响应：200 `{"success":true,"data":{"available","version","baseUrl","error"}}`

```bash
curl $BASE/api/v1/initialization/ollama/status -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/initialization/ollama/models

用途：列出本地 Ollama 模型。权限：Viewer+。

响应：200 `{"success":true,"data":{"models":[...]}}`

```bash
curl $BASE/api/v1/initialization/ollama/models -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/initialization/ollama/models/check

用途：批量检查模型是否已存在。权限：平台管理。请求体：`{"models":["llama3"]}`（`binding:"required"`）。

响应：200 `{"success":true,"data":{"models":{"llama3":true}}}`

```bash
curl -X POST $BASE/api/v1/initialization/ollama/models/check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"models":["llama3"]}'
```

### POST /api/v1/initialization/ollama/models/download

用途：拉取 Ollama 模型（异步任务）。权限：平台管理。请求体：`{"modelName":"llama3"}`（`binding:"required"`）。

响应：200 `{"success":true,"data":{"taskId","modelName","status","progress"}}`

```bash
curl -X POST $BASE/api/v1/initialization/ollama/models/download -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"modelName":"llama3"}'
```

### GET /api/v1/initialization/ollama/download/progress/:taskId

用途：下载任务进度。权限：Viewer+。

响应：200 `{"success":true,"data":{id,modelName,status,progress,message,startTime,endTime}}`

```bash
curl $BASE/api/v1/initialization/ollama/download/progress/task-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/initialization/ollama/download/tasks

用途：全部下载任务列表。权限：Viewer+。

响应：200 `{"success":true,"data":[DownloadTask]}`

```bash
curl $BASE/api/v1/initialization/ollama/download/tasks -H "Authorization: Bearer $TOKEN"
```

### 模型连通性检测（均 POST，权限：平台管理）

请求体统一为 `ModelTestRequest`：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `source` | string | 否 | 默认 `remote` |
| `modelName` | string | 是 | 模型名 |
| `baseUrl` / `apiKey` / `appSecret` | string | 否 | 连接参数 |
| `provider` / `interfaceType` | string | 否 | 厂商/接口类型 |
| `dimension` | int | 否 | embedding 维度 |
| `customHeaders` / `extraConfig` | map | 否 | 扩展 |
| `modelId` | string | 否 | 从已存模型取密钥 |

| 端点 | 用途 | 响应 data |
| --- | --- | --- |
| `POST /api/v1/initialization/remote/check` | LLM 远程连通性 | `{available,message}` |
| `POST /api/v1/initialization/embedding/test` | Embedding 测试 | `{available,message,dimension}` |
| `POST /api/v1/initialization/rerank/check` | Rerank 测试 | `{available,message}` |
| `POST /api/v1/initialization/asr/check` | ASR 测试 | `{available,message}` |

```bash
curl -X POST $BASE/api/v1/initialization/remote/check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"modelName":"gpt-4o-mini","baseUrl":"https://api.openai.com/v1","apiKey":"sk-..."}'
```

### POST /api/v1/initialization/multimodal/test

用途：多模态（VLM+图床）端到端测试。权限：平台管理。multipart 字段：`image`（必填）、`vlm_model`、`vlm_base_url`（必填）、`vlm_api_key`、`vlm_interface_type`、`storage_type`（`local|s3`，必填；S3 连接参数取自服务端 `S3_*` 环境变量）、`chunk_size`、`chunk_overlap`、`separators`。

响应：200 `{"success":true,"data":{"success","caption","ocr","processing_time"}}`

```bash
curl -X POST $BASE/api/v1/initialization/multimodal/test -H "Authorization: Bearer $TOKEN" \
  -F 'image=@demo.png' -F 'vlm_model=qwen-vl' -F 'vlm_base_url=http://x' -F 'storage_type=local'
```

### POST /api/v1/initialization/extract/text-relation

用途：文本图谱抽取测试。权限：Admin+（作用于用户内容，不随集中管控上收）。请求体：`text`（必填，≤5000 字符）、`tags`（必填，至少一个）、`model_id`（必填）。

响应：200 `{"success":true,"data":{"nodes":[GraphNode],"relations":[GraphRelation]}}`

```bash
curl -X POST $BASE/api/v1/initialization/extract/text-relation -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"text":"小明在某公司工作","tags":["人物","公司"],"model_id":"m-1"}'
```

### POST /api/v1/initialization/extract/fabri-tag

用途：生成示例标签。权限：Admin+。无请求体。

响应：200 `{"success":true,"data":{"tags":[...]}}`

```bash
curl -X POST $BASE/api/v1/initialization/extract/fabri-tag -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/initialization/extract/fabri-text

用途：按标签生成示例文本。权限：Admin+。请求体：`{"tags":[...],"model_id":"m-1"}`（model_id 必填）。

响应：200 `{"success":true,"data":{"text":"..."}}`

```bash
curl -X POST $BASE/api/v1/initialization/extract/fabri-text -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"model_id":"m-1","tags":["人物"]}'
```

## 评估（/api/v1/evaluation）

Handler: `internal/handler/evaluation.go`。API key：`run_evaluations`/full。

### POST /api/v1/evaluation

用途：发起评估任务（驱动 LLM 调用，产生费用）。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `dataset_id` | string | 否 | 数据集 ID，省略时为 `default` |
| `knowledge_base_id` | string | 否 | 参照的 KB：评估总是新建一个名为 `evaluation` 的知识库，沿用该 KB 的 Embedding 与摘要模型；省略时取空间里找到的 Embedding 与对话模型，找不到则失败 |
| `chat_id` | string | 否 | 对话模型 ID，省略时取空间里的一个 `KnowledgeQA` 模型，找不到则失败 |
| `rerank_id` | string | 否 | Rerank 模型 ID，省略时取空间里的一个 `Rerank` 模型（可以没有） |

响应：200 `{"success":true,"data":{"task":{EvaluationTask},"params":{...}}}`。`task` 字段：`id`、`tenant_id`、`dataset_id`、`start_time`、`status`（0 等待、1 运行中、2 成功、3 失败）、`err_msg`、`total`、`finished`；`params` 是本次评估实际使用的检索与生成参数。

```bash
curl -X POST $BASE/api/v1/evaluation -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"knowledge_base_id":"kb-1","chat_id":"m-1"}'
```

### GET /api/v1/evaluation

用途：查询评估结果。权限：Viewer+。查询参数：`task_id`（必填）。

响应：200 `{"success":true,"data":{"task":{...},"params":{...},"metric":{...}}}`。`metric` 在有结果后出现：`retrieval_metrics`（`precision`、`recall`、`ndcg3`、`ndcg10`、`mrr`、`map`）与 `generation_metrics`（`bleu1`、`bleu2`、`bleu4`、`rouge1`、`rouge2`、`rougel`）。指标含义与数据集格式见[评估能力](../03-features/15-evaluation.md)。

```bash
curl "$BASE/api/v1/evaluation?task_id=task-1" -H "Authorization: Bearer $TOKEN"
```
