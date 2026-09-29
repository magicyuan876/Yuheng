# 常见问题

## 1. 如何查看日志？
```bash
docker compose logs -f app docreader postgres
```

## 2. 如何启动和停止服务？
```bash
# 启动服务
./scripts/start_all.sh

# 停止服务
./scripts/start_all.sh --stop

# 清空数据库
./scripts/start_all.sh --stop && make clean-db
```

## 3. 服务启动后无法正常上传文档？

通常是Embedding模型和对话模型没有正确被设置导致。按照以下步骤进行排查

1. 查看`.env`配置中的模型信息是否配置完整，其中如果使用ollama访问本地模型，需要确保本地ollama服务正常运行，同时在`.env`中的如下环境变量需要正确设置:
```bash
# LLM Model
INIT_LLM_MODEL_NAME=your_llm_model
# Embedding Model
INIT_EMBEDDING_MODEL_NAME=your_embedding_model
# Embedding模型向量维度
INIT_EMBEDDING_MODEL_DIMENSION=your_embedding_model_dimension
# Embedding模型的ID，通常是一个字符串
INIT_EMBEDDING_MODEL_ID=your_embedding_model_id
```

如果是通过remote api访问模型，则需要额外提供对应的`BASE_URL`和`API_KEY`:
```bash
# LLM模型的访问地址
INIT_LLM_MODEL_BASE_URL=your_llm_model_base_url
# LLM模型的API密钥，如果需要身份验证，可以设置
INIT_LLM_MODEL_API_KEY=your_llm_model_api_key
# Embedding模型的访问地址
INIT_EMBEDDING_MODEL_BASE_URL=your_embedding_model_base_url
# Embedding模型的API密钥，如果需要身份验证，可以设置
INIT_EMBEDDING_MODEL_API_KEY=your_embedding_model_api_key
```

当需要重排序功能时，需要额外配置Rerank模型，具体配置如下：
```bash
# 使用的Rerank模型名称
INIT_RERANK_MODEL_NAME=your_rerank_model_name
# Rerank模型的访问地址
INIT_RERANK_MODEL_BASE_URL=your_rerank_model_base_url
# Rerank模型的API密钥，如果需要身份验证，可以设置
INIT_RERANK_MODEL_API_KEY=your_rerank_model_api_key
```

2. 查看主服务日志，是否有`ERROR`日志输出

## 4. 没有图片或者显示无效的图片链接？

当使用多模态功能时，如果遇到图片无法显示或显示无效链接的问题，请按照以下步骤排查：

### 1. 确认多模态功能已正确配置

在知识库设置中开启**高级设置 - 多模态功能**，并在界面中配置相应的多模态模型。

### 2. 确认对象存储可用

图片由对象存储保存。默认的 `STORAGE_TYPE=local` 不需要额外服务；如果配置的是 `STORAGE_TYPE=s3`，请确认 S3 兼容服务可达。使用 compose 自带的 RustFS 时：

```bash
# 启动 RustFS（默认只绑定 127.0.0.1）
docker compose --profile rustfs up -d

# 或者启动完整服务（包括 RustFS、Neo4j 等）
docker compose --profile full up -d
```

对应的 `.env` 配置：

```bash
STORAGE_TYPE=s3
S3_ENDPOINT=http://rustfs:9000
S3_REGION=us-east-1
S3_BUCKET_NAME=yuheng
S3_ACCESS_KEY=rustfsadmin        # 与 RUSTFS_ACCESS_KEY 相同
S3_SECRET_KEY=rustfsadmin        # 与 RUSTFS_SECRET_KEY 相同
S3_ADDRESSING_STYLE=path
```

`rustfs:9000` 属于内网地址，需要被 `SSRF_WHITELIST` / `SSRF_WHITELIST_EXTRA` 放行（compose 已默认放行 `rustfs` 服务名）。

### 3. 检查 Bucket 与凭据

1. 在 **设置 → 存储后端** 里对当前配置点「测试连接」，或调用 `POST /system/storage-engine-check`，根据返回的 `message` 定位问题
2. 确认 `S3_BUCKET_NAME` 对应的 bucket 可读写。bucket 不存在时，首次使用会自动创建
3. 使用云厂商（阿里云 OSS、腾讯云 COS、火山引擎 TOS、华为云 OBS）时，`S3_ADDRESSING_STYLE` 必须设为 `virtual`，否则请求会被拒绝

**重要提示**：
- Bucket 名称不要包含特殊字符（包括中文），建议使用小写字母、数字和连字符
- `S3_ACCESS_KEY` 与 `S3_SECRET_KEY` 要么都填、要么都不填；都不填时使用 AWS 默认凭据链

### 4. 图片链接无法从其他设备访问

图片默认以内部引用（`resource://`）保存，浏览器通过带登录态的 `/files` 代理读取，不依赖对象存储对外可达。如果需要生成外部可访问的直链，请配置 `APP_EXTERNAL_URL`，或让 S3 endpoint 本身公网可达。`S3_ENDPOINT` 使用 `localhost` 或容器内地址时，其他设备无法直接访问该地址。

## 5. 平台兼容性说明

**重要提示**：`OCR_BACKEND=paddle` 模式在部分平台上可能无法正常运行。如果遇到 PaddleOCR 启动失败的问题，请选择以下解决方案

### 方案一：关闭 OCR 识别

在 `docker-compose.yml` 文件的 `docreader` 服务中删除 `OCR_BACKEND` 配置，然后重启 docreader 服务

**注意**：设置为 `no_ocr` 后，文档解析将不会使用 OCR 功能，这可能会影响图片和扫描文档的文字识别效果。

### 方案二：使用外部 OCR 模型（推荐）

如果需要 OCR 功能，可以使用外部的视觉语言模型（VLM）来替代 PaddleOCR。在 `docker-compose.yml` 文件的 `docreader` 服务中配置：

```yaml
environment:
  - OCR_BACKEND=vlm
  - OCR_API_BASE_URL=${OCR_API_BASE_URL:-}
  - OCR_API_KEY=${OCR_API_KEY:-}
  - OCR_MODEL=${OCR_MODEL:-}
```

然后重启 docreader 服务

**优势**：使用外部 OCR 模型可以获得更好的识别效果，且不受平台限制。

## 6. 页面里刚保存的配置几秒后又消失了？

这类问题通常不是配置真的被系统清掉了，而是浏览器代理、缓存或插件干扰导致前端读到了异常响应，页面随后又被旧状态覆盖。

建议按下面顺序排查：

1. 先关闭浏览器代理、抓包工具、自动改写请求的插件，再重新打开页面。
2. 确认浏览器没有把 `localhost` 或当前访问域名走代理；如果配置了 PAC，请将 `localhost`、`127.0.0.1` 和实际部署域名加入直连名单。
3. 强制刷新页面，或直接使用无痕窗口重新登录后再保存一次配置。
4. 打开浏览器开发者工具的 `Network` 面板，确认保存配置相关请求返回的是最新内容，且没有被代理改写、缓存命中或重定向到其他环境。
5. 如果是调试模式部署，可尝试重启 `app` 服务后再验证一次：

```bash
docker compose restart app
```

如果重启后短时间恢复正常，但再次访问又出现相同现象，仍应优先检查浏览器代理、缓存和多环境串连问题，而不是直接判断为后端配置丢失。

## 7. SSRF 校验白名单（`SSRF_WHITELIST`）

可选配置。在 `.env` 中设置 `SSRF_WHITELIST`，用于在 URL 校验等环节将指定目标加入白名单，从而绕过常规 SSRF 限制。值为逗号分隔的多条规则，每条可以是：

- **精确域名**：如 `api.internal`
- **通配域名**：如 `*.example.com`
- **IPv4**：如 `203.0.113.5`
- **IPv6**：如 `2001:db8::1`（不要带方括号）
- **CIDR**：如 `10.0.0.0/8`、`2001:db8::/32`

列入白名单的地址会在 URL 校验等处绕过常规 SSRF 规则，**生产环境请谨慎配置**，仅加入确实需要且可信的目标。

示例（与 `.env.example` 一致，可按需取消注释并修改）：

```bash
# SSRF_WHITELIST=internal.service,*.corp.example,172.16.0.0/12,2001:db8::1,fd00::/8
```


## 8. 如何开启和查看 Langfuse 可观测性追踪？

Yuheng 支持通过 Langfuse 对 RAG 检索管道、大模型 Token 消耗以及异步任务流水线进行全链路追踪。

**开启步骤**：
1. 准备一个可用的 Langfuse 实例（支持云端版或私有部署版）。
2. 在 `.env` 文件中配置以下环境变量：
```bash
LANGFUSE_PUBLIC_KEY=pk-lf-...
LANGFUSE_SECRET_KEY=sk-lf-...
LANGFUSE_HOST=https://cloud.langfuse.com # 或你的私有部署地址
```
3. 重启服务后，系统会自动对所有支持的模型调用和问答请求进行追踪，你可以在 Langfuse 的 Traces 面板中直观地看到每次对话和后台任务的详细执行瀑布图与 Token 统计。

## 9. 什么是 Wiki 模式？如何使用？

Wiki 模式会根据原始文档自动生成并维护一套结构化、相互链接的 Markdown Wiki 知识库，从而实现复杂知识的体系化沉淀和图谱化。

**使用方法**：
1. 进入指定**知识库的设置** -> **索引策略 (Indexing Strategy)**。
2. 开启 **Wiki** 索引功能（可同时结合开启**知识图谱**）。
3. 当你向该知识库上传文档时，系统会自动触发异步任务，通过大模型提取文档中的实体与核心概念，并自动生成结构化的 Wiki 页面及页面间的知识图谱链接。
4. 你可以在该知识库的“Wiki”标签页中，使用专用的 Wiki 浏览器查阅、管理页面，并通过可视化的知识图谱查看不同内容之间的关联关系。

## 10. 升级到 0.6.0 后，原本能做的操作变成了「权限不足」？

0.6.0 引入了空间内 RBAC（角色矩阵 + 资源归属），所有写入接口都会按角色 + `creator_id` 鉴权。常见现象：

- **看得到但点不动**：你大概率是该资源的 `Viewer` 或非创建者的 `Contributor`，UI 已经把写操作隐藏/置灰。检查 **用户菜单 → 当前工作区** 角色徽章。
- **共享空间里的 KB**：他人共享给你的 KB 默认按 `Viewer` 看待；要写需要在源空间里被授予 `Admin+`。
- **API Key 调用**：`X-API-Key` 合成虚拟用户固定为所属空间的 `Admin`（仅删除空间需 `Owner`），脚本一般无需迁移。
- **跨空间超管**：要 `User.CanAccessAllTenants=true` 且 `enable_cross_tenant_access=true`，并通过 `X-Tenant-ID` 切空间。

如需临时回退到「仅审计、不拦截」灰度窗口，可在配置里设置 `tenant.enable_rbac=false`（或环境变量 `YUHENG_TENANT_ENABLE_RBAC=false`）。完整的角色矩阵和归属链请见 [`docs/RBAC说明.md`](./RBAC说明.md)。

## 11. 为什么登录后没有自动回到上次的工作区？

升级到 0.6.0 后系统会记住「最后活跃工作区」并在登录后自动恢复。若仍未恢复，通常是：

1. 浏览器清理了 LocalStorage / 切换了浏览器；
2. 你最后访问的那个工作区已经把你移除（`/leave` 或被管理员剔除）— 系统会回退到默认空间；
3. JWT 中携带了 `tenant_id` 但已无效 — 退出重登录即可。

## 12. 如何让多人协作时正确分配权限？

按照 [`docs/RBAC说明.md`](./RBAC说明.md) 的角色矩阵：

- 只读用户 → `Viewer`
- 普通成员（上传文档、维护「自己」的 KB）→ `Contributor`
- 运维人员（管理共享模型、向量库、解析器等基础设施）→ `Admin`
- 空间所有者（拥有删除空间权限；每空间至少一位，可以有多位，最后一位不能被降级或移除）→ `Owner`

如果你希望开启「invite-only」（不允许自助注册到本空间），可在空间设置里打开邀请制，并通过「邀请」入口签发邀请码或链接。

## 13. 文档解析卡在「处理中」/ 解析追踪时间线打不开怎么办？

0.6.1 起每个文档解析都会记录一棵 Langfuse 风格的 Span 树（`knowledge_processing_spans` 表），可在知识库卡片菜单或卡片上的「Trace」入口打开侧边时间线，逐阶段查看进度。常见情况：

- **文档长时间停在「处理中」**：先打开时间线看是哪个阶段没有推进（解析 / 切分 / 向量化 / 后处理）。0.6.1 已修复多数「卡死」场景，并加入看门狗轮询；如确认是某次解析挂死，可在时间线面板点击「中止解析」，文档会进入 finalizing 后处理状态后结束。
- **时间线一直显示「更新中」但无数据**：通常是轮询请求静默失败（网络 / 反向代理截断 SSE）。0.6.1 会显式暴露轮询失败，刷新页面或检查 Nginx 是否缓冲了响应即可。
- **升级后没有时间线数据**：确认数据库迁移 `000055_knowledge_processing_spans`、`000056_knowledge_pending_subtasks` 已执行（服务启动会自动迁移）。

## 14. 能否改用 Elasticsearch / OpenSearch / Milvus / Qdrant 等外部向量库？

社区版只支持一种检索引擎：带 ParadeDB（`pg_search`，BM25）与 pgvector 的 PostgreSQL，即 `RETRIEVE_DRIVER=postgres`。Elasticsearch、OpenSearch、Milvus、Weaviate、Qdrant、Doris、腾讯云 VectorDB 的驱动已从社区版移除，`docker-compose.yml` 也不再带对应服务与环境变量；「设置 → 向量库」的注册机制仍在，但社区版没有可注册的引擎。

如果启动时报错，提示 PostgreSQL 缺少 `vector` 或 `pg_search` 扩展，说明所连数据库不是 `docker-compose.yml` 使用的 ParadeDB 镜像。处理办法二选一：

- 直接使用 `docker-compose.yml` 中的 ParadeDB 镜像（已内置两个扩展）；
- 在自有 PostgreSQL 上自行安装 pgvector 与 pg_search。注意云厂商托管的 PostgreSQL 通常无法安装 `pg_search`，此时不能使用托管库。

## 15. 内置模型（builtin models）如何用 YAML 声明式管理？

0.6.1 起平台内置模型由 `config/builtin_models.yaml` 声明式驱动，支持 `${ENV}` 变量插值，并通过 `managed_by` 字段与漂移巡检保持数据库与 YAML 一致。常见问题：

- **改了 YAML 不生效**：内置模型在服务启动时做生命周期对账（drift sweep）；确认重启了服务，且条目通过了 schema 校验（ID 长度、必填字段）。
- **Docker 下环境变量未注入**：`builtin_models` 依赖 `env_file` 数组形式注入变量，确认 compose 中按数组形式挂载了 `.env`。
- 参考样例：`config/builtin_models.yaml.example`。

## 16. 系统管理员（System Admin）与平台设置怎么用？

0.6.1 引入了系统管理员与统一平台设置面板（含平台审计日志），与空间内 RBAC 区分：系统管理员管理的是「平台级」配置，而非单个空间内的资源。首次启用需通过系统管理员 bootstrap 流程晋升首个管理员；撤销管理员权限有安全防护（避免误撤导致无人可管）。相关迁移为 `000053_system_admin_and_settings`。

## 17. 上传时如何自定义解析配置（process_config）？

0.6.2 起，文件 / URL / 文件夹上传可携带 `process_config`（`KnowledgeProcessOverrides`），在**本次批次**内覆盖知识库默认的解析引擎、分块、多模态（VLM / ASR）、问题生成、图谱抽取等设置，而不会改动 KB 全局配置。Web UI 在上传前会弹出确认对话框供调整；API 与 `yuheng doc upload` 传同名 JSON 即可。

- **与 KB 默认配置的关系**：未传的字段沿用 KB 默认值；`graph_enabled` 仅在 `extract_config.enabled` 为 true 时生效。
- **重新解析**：`POST /knowledge/:id/reparse` 可在 body 中传 `process_config` 以新配置重跑解析，覆盖项会写入 `knowledge.metadata.process_overrides`。
- **图片 / 音频校验**：批次含图片时需 KB 已配置 VLM；含音频时需已配置 ASR，否则上传会被拒绝。
- 详见 [`docs/api/knowledge.md`](./api/knowledge.md)。

## 18. pgvector 检索变慢或刚升级后需要做什么？

0.6.2 新增迁移 `000059_embeddings_hnsw_1024`，为 **1024 维** embedding（如 bge-m3）在 PostgreSQL pgvector 上创建 HNSW 索引。服务启动会自动执行迁移；若你使用其他维度，该索引可能不适用，需按自身 embedding 维度另行调优。升级后首次大批量入库期间索引构建可能占用额外 I/O，属正常现象。

## 19. 文档如何设置多个标签？

0.6.3 将文档标签从单选升级为**多标签**（迁移 `000063_knowledge_multi_tags`）。在知识库列表可为文档打多个标签，侧边栏支持按标签筛选；**标签管理**抽屉可批量维护标签。API 上传 / 更新知识时传 `tag_ids` 数组（取代旧的单 `tag_id`）。

## 20. 如何批量重新解析文档？

在知识库文档列表框选多篇文档后，使用批量操作栏的 **重新解析**；也可调用 `POST /knowledge/batch-reparse`，body 可含 `ids` 与可选 `process_config`。任务异步入队，UI 会在入队后刷新状态。单篇仍可用 `POST /knowledge/:id/reparse`。

## 21. RSS 数据源如何配置？

0.6.3 新增 **RSS / Atom** 连接器。在知识库 **设置 → 数据源** 中选择 RSS，填写 Feed URL 与同步策略即可全量 / 增量拉取正文入库。若部分条目失败，同步日志会展示 partial failure 详情；编辑数据源保存配置**不会**自动触发同步，需手动点同步。

## 22. Embedding 维度如何覆盖？

在 **设置 → 模型** 编辑 Embedding 模型时可填写 **dimensions** 覆盖值（如 1024、1536）。0.6.3 修复了部分提供商请求未携带 `dimensions` 的问题（#1654）。若向量库索引维度与模型不一致，检索可能异常，请保持 KB 绑定向量库与模型维度一致。

## 23. 如何创建并限制权限范围 API Key？

Yuheng 采用**权限范围 API Key 与 Principal 模型**（迁移 `000064_principal_model`、`000065_tenant_api_keys`）。API Key 不再等同于某个人类用户，而是独立的 Principal，携带显式角色与能力（capability）授权：

- 在 **设置 → API 集成**（Owner 可见）中创建 Key，可勾选能力（如 `manage_kbs` 覆盖 KB 全生命周期、`manage_storage_backends` 等），并可限制到指定知识库。
- Key 的 `last_used_at` 按节流更新，避免高频写库。
- 路由级守卫会拒绝越权访问；管理类接口对 API Key Principal 默认拒绝，请为集成使用具备对应能力的 Key，而非全权 Key。
- MCP OAuth 与嵌入会话按 Principal 隔离，不同集成之间互不串号。

### 如何用一个 API Key 自动化管理多个空间？

SystemAdmin 可在 **系统管理 → 平台 API Key** 创建 `scope_type=platform` 的 Key。平台 Key 不绑定单一空间：调用普通空间 API 时必须携带 `X-Tenant-ID`，并继续受原有 capability 和知识库范围守卫约束；调用开放的系统控制面接口则需要对应的 `system_*` capability。平台 Key 不支持 `full_access`，也不能创建、轮换或吊销其他平台 Key。

## 24. 一个空间如何绑定多个对象存储实例？

Yuheng 支持**多实例存储后端**（迁移 `000068_storage_backends`）。一个空间可注册多个存储实例（`local` / `s3`，MinIO、RustFS、AWS S3、阿里云 OSS、腾讯云 COS 等都用 `s3` 接入），不同知识库绑定到不同实例，空间维度还有一个默认实例：

- 在 **设置 → 存储后端** 创建/测试/设为默认（需 Admin+；API Key 需 `manage_storage_backends` 能力）。
- 未显式绑定的新知识库使用空间默认实例；响应中的 `access_key_id` / `secret_access_key` 会被掩码，更新时提交掩码占位符不会覆盖库中真实凭据。
- 若创建知识库时提示存储引擎不可用，请确认目标 provider 在 `STORAGE_ALLOW_LIST` 允许范围内。详见 [`docs/api/storage-backend.md`](./api/storage-backend.md)。

## 25. 后台解析/入库任务积压或需要排查失败任务怎么办？

系统管理员可使用**运行时任务队列面板**与 **Worker 池治理**。文档处理从单一聚合池改为分阶段独立池（core / 后处理 / enrichment / maintenance）+ 弹性共享池，Wiki 独立治理：

- 在 **系统设置 → 运行时队列** 查看队列深度、按模型并发统计、失败任务详情，并可手动重试。
- 可通过 `YUHENG_ASYNQ_*_CONCURRENCY` 与 `asynq.*_concurrency` 系统设置调整各池并发（需重启服务）；`model.max_concurrency` 用于约束单模型后台并发。
- 详见 [`docs/worker-pool-governance.md`](./worker-pool-governance.md)。注意：Worker 并发只是调度预算，仍受模型配额、DocReader 容量、向量库与数据库连接数限制。

## 26. 对话中如何临时上传图片/文档做一次性问答？

支持**会话级临时附件**（迁移 `000070_temporary_documents`）。在对话输入区上传图片或文档，系统异步解析后仅用于当前会话的问答，不会写入知识库。图片与附件共享一个合并数量上限；附件内容会在多轮对话中保留。

## 27. 如何为 Redis 启用 TLS？

支持 Redis 的 **TLS 连接**（#1930）。按环境变量启用 TLS 后，启动日志会打印 TLS 配置状态便于确认。若连接失败，请核对证书/CA 配置与 Redis 服务端是否要求 TLS。

## 28. 如何使用火山引擎 Rerank / 智谱 AI 网络搜索？

支持两个供应商：

- **火山引擎 Rerank**：在 **设置 → 模型** 中添加 Rerank 模型并选择火山引擎。当单次请求文档数超过 API 上限时，客户端会自动分批发送并合并结果。vLLM Rerank 现默认不再发送 `truncate_prompt_tokens` 以提升兼容性。
- **智谱 AI 网络搜索**：在 **设置 → 网络搜索** 中选择智谱 AI 作为搜索供应商并填写凭据即可，用于问答联网检索。

## 29. 官方文档在哪里看？如何本地或独立部署文档站？

完整的官方产品文档位于仓库 [`website-docs/`](../website-docs/README.md) 目录，按「入门 → 架构 → 功能 → API → 客户端 → 开发」六个板块组织，覆盖约 220 个 API 端点、约 330 个环境变量（含 `.env.example` 中的注释示例）与 7 大扩展点。

该目录同时是一个 VitePress 站点，两种使用方式：

```bash
# 本地预览
cd website-docs && npm install && npm run dev

# 独立容器部署（容器内 Nginx 监听 8081）
docker build -t yuheng-docs website-docs
docker run -d -p 8081:8081 yuheng-docs
```

站点的版本号在构建时自动读取仓库根目录的 `VERSION` 文件，因此升级版本后无需手动改文档。若某处截图显示为虚线占位框，说明 `website-docs/public/screenshots/` 下缺少同名图片，补图即可生效，不需要改 Markdown。

`website-docs/sample-data/` 下还提供了 4 份 Markdown 样例文档与 1 份 FAQ 导入 JSON，可以直接用来跑一遍「建库 → 上传 → 问答」；`examples/mcp-demo/` 是一个可直接运行的本地 MCP 服务示例。

## 30. 文件夹上传后文档标题变成了一长串路径？

旧版本的行为是：文件夹上传会把相对目录塞进 `file_name`，导致列表里显示整条路径，也无法按文件夹筛选。

当前版本把路径拆到独立的 `folder_path` 字段（迁移 `000079_knowledge_folder_path`），并**自动回填历史数据**，因此已有知识库同样会呈现正确的文件夹树，无需重新上传。文档列表左侧会出现文件夹树，可以像文件管理器一样浏览、重命名文件夹，也可以通过行内的文件夹选择器把文档重新归档到其他文件夹。单独上传（非文件夹）的文档统一挂在树的根目录下。

## 31. 分块内容不准确，可以手动修改吗？改完会重新建索引吗？

可以。支持在界面上直接编辑检索分块（迁移 `000078_chunk_editing_and_custom_metadata`）：

- 每次编辑都会把改动前的版本存入 `chunk_revisions`，可在「分块编辑历史」里逐版本查看 diff 并一键回滚。
- **编辑保存后会自动重建该分块的索引**（`index_status` 字段跟踪重建状态），无需手动 reparse。
- 分块的生成问题可以单独增删改与重新生成，且在内容编辑后仍会保留。
- 注意：重新解析（reparse）整篇文档会按新的解析结果重建分块，此前的人工编辑不会被保留，请谨慎操作。

## 32. Wiki 页面被自动流水线覆盖了，能找回旧版本吗？

能。Wiki 页面提供版本历史（迁移 `000075_wiki_page_revisions`）：页面每次被覆盖前都会留存一份快照，在 Wiki 浏览器右上角打开「版本历史」抽屉即可查看完整历史、行级 diff，并一键回滚到任意版本。每个版本都会记录来源（`pipeline` 流水线 / `user` 手动编辑 / `revert` 回滚），便于判断是谁改的。页面也支持在浏览器内直接手动编辑。

另外，Wiki 浏览器里重复的操作日志已移除（迁移 `000077_remove_wiki_log`），Wiki 的变更记录统一并入**知识库活动流**查看。

## 33. 第三方 App 拿到的图片链接是 `resource://...` 无法显示，怎么办？

默认情况下 API 返回的是内部句柄 `resource://<handle>`，客户端需要再调用带鉴权的 `/files` 代理才能取到图。可开启直链模式，让接口直接返回可加载的 http(s) 链接：

- **单次请求**：在 URL 上加 `?resource_urls=public`。
- **整个部署**：设置环境变量 `RESOURCE_URL_MODE=public`。

注意事项：

- 直链依赖 `APP_EXTERNAL_URL`（或存储后端本身公网可达）才能生成；无法生成时该引用会保持 `resource://` 原样，客户端仍可回退到 `/files`。
- `public` 会为每个被引用文件签发**限时匿名可读**链接（Yuheng 侧 2 小时，存储后端预签名的时长由存储决定），请评估是否符合你的安全要求。
- 限定了知识库范围的 API Key **始终返回 handle**，不受该变量影响。
- 建议同时配置 `SYSTEM_AES_KEY`，以便复用 grant 行、稳定直链 URL 并降低读接口的写入压力。

详见 [API 文档 · 文件与图片引用](./api/README.md)。

## 34. 使用 AWS S3 但不想在配置里写 AK/SK？

支持 **AWS SDK 默认凭据链**（#2008）：把 `S3_ACCESS_KEY` 与 `S3_SECRET_KEY` **同时留空**即可，SDK 会依次尝试 EC2/ECS/EKS 实例角色、IRSA / Web Identity、环境变量与共享配置文件。注意两者必须同时填写或同时留空，只填一个会报配置错误。`S3_ENDPOINT` 也可留空，此时使用 `S3_REGION` 对应的 AWS 标准端点。

## 35. MCP Server 用 `uvx` 启动失败，或者应该装哪个包？

Yuheng 的 MCP Server 包名为 **`yuheng-mcp`**，命令行入口是 `yuheng-mcp-server` / `yuheng-server`（源码运行：`uv run --project mcp-server yuheng-mcp-server`）。

服务端 Agent 能力剥离后，MCP Server 的工具面聚焦知识平台能力，工具总数为 23 个（知识库 / 文档 / 检索 / RAG 问答 / 分块 / Wiki / 模型管理），已移除 Agent 相关的工具解析逻辑；传输支持 stdio / SSE / HTTP。配置见 [`mcp-server/MCP_CONFIG.md`](../mcp-server/MCP_CONFIG.md)。

行为变化提醒：工具执行失败时，MCPServer 2.x 返回 `CallToolResult(isError=True)`，不再像旧版低层 API 那样以成功响应返回 `"Error executing …"` 文本前缀。只解析 `content[0].text` 的客户端通常无感，依赖 `isError` 标志的集成方行为会更符合 MCP 规范。

## P.S.
如果以上方式未解决问题，请在issue中描述您的问题，并提供必要的日志信息辅助我们进行问题排查
