# 常见问题

## 1. 如何查看日志？
```bash
docker compose logs -f app docreader postgres
```

## 2. 如何启动和停止服务？
```bash
# 启动服务（本版本不发布镜像，从源码构建；先执行 ./scripts/build_frontend_dist.sh）
docker compose up -d --build

# 停止服务
docker compose down

# 清空数据库
docker compose down && make clean-db
```

也可以用 `scripts/start_all.sh`：它默认不拉取镜像、在本机构建（`--pull` 才会拉取，而本版本没有可拉取的镜像）。完整的部署方式见[安装部署](../website-docs/01-getting-started/02-installation.md)。

## 3. 服务启动后无法正常上传文档？

通常是Embedding模型和对话模型没有正确被设置导致。按照以下步骤进行排查

1. 在「设置 → 模型管理」里确认已经添加了对话（LLM）模型和向量（Embedding）模型，并且新建知识库时选用了它们。用「测试连接」确认模型可用；如果通过 Ollama 访问本地模型，需要确保 Ollama 服务正常运行，并且容器内能访问到它（默认地址 `http://host.docker.internal:11434`，即 `OLLAMA_BASE_URL`）。需要重排序功能时，再额外添加一个 Rerank 模型。

2. 查看主服务日志，是否有`ERROR`日志输出

## 4. 没有图片或者显示无效的图片链接？

当使用多模态功能时，如果遇到图片无法显示或显示无效链接的问题，请按照以下步骤排查：

### 1. 确认多模态功能已正确配置

在知识库设置「索引与解析」分组的**图像处理**页开启多模态，并选好视觉（VLM）模型。

### 2. 确认对象存储可用

图片由对象存储保存。默认的 `STORAGE_TYPE=s3` 使用 compose 自带的 RustFS，`docker compose up -d` 会一并启动它（默认只绑定 127.0.0.1），无需额外配置。如果改用外部 S3 兼容服务，请确认它可达；设为 `STORAGE_TYPE=local` 则存到本机目录。

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

1. 在 **设置 → 存储引擎** 里对当前后端点「测试」，或调用 `POST /api/v1/storage-backends/:id/test`（未保存的配置用 `POST /api/v1/storage-backends/test`），根据返回的 `error` 定位问题。存储配置的完整说明见[存储后端](../website-docs/03-features/19-storage-backends.md)
2. 确认 `S3_BUCKET_NAME` 对应的 bucket 可读写。bucket 不存在时，首次使用会自动创建
3. 使用云厂商（阿里云 OSS、腾讯云 COS、火山引擎 TOS、华为云 OBS）时，`S3_ADDRESSING_STYLE` 必须设为 `virtual`，否则请求会被拒绝

**重要提示**：
- Bucket 名称不要包含特殊字符（包括中文），建议使用小写字母、数字和连字符
- `S3_ACCESS_KEY` 与 `S3_SECRET_KEY` 要么都填、要么都不填；都不填时使用 AWS 默认凭据链

### 4. 图片链接无法从其他设备访问

图片默认以内部引用（`resource://`）保存，浏览器通过带登录态的 `/files` 代理读取，不依赖对象存储对外可达。如果需要生成外部可访问的直链，请配置 `APP_EXTERNAL_URL`，或让 S3 endpoint 本身公网可达。`S3_ENDPOINT` 使用 `localhost` 或容器内地址时，其他设备无法直接访问该地址。详见[图片与文件的对外访问](../website-docs/03-features/21-file-access.md)。

## 5. 扫描件或图片里的文字没有被识别出来？

docreader 本身不做 OCR，图片与扫描件的文字识别由主服务调用视觉模型（VLM）完成。旧版本里 docreader 的 `OCR_BACKEND`（PaddleOCR / `no_ocr` / `vlm`）等变量已经不存在，设置它们没有效果。按下面检查：

1. 知识库开启了**图像处理**并配置了视觉模型（见上一条）；
2. 扫描件或网页打印的 PDF，在上传确认对话框里勾选「按扫描件解析 PDF」，逐页渲染后由视觉模型识别；
3. 版式复杂的 PDF 可以在知识库「解析引擎」里换引擎（MarkItDown、OpenDataLoader，或配置好的 MinerU / PaddleOCR-VL 服务）。

详见[文档解析服务](../website-docs/03-features/03-document-parsing.md)。

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

列入白名单的地址会在 URL 校验等处绕过常规 SSRF 规则，**生产环境请谨慎配置**，仅加入确实需要且可信的目标。系统管理员也可以在「设置 → 系统管理 → 系统设置」里修改 `ssrf.whitelist`，立即生效，且数据库里的值优先于环境变量；compose 默认通过 `SSRF_WHITELIST_EXTRA=searxng,rustfs` 放行自带服务，这个变量只能由部署方设置。

示例（与 `.env.example` 一致，可按需取消注释并修改）：

```bash
# SSRF_WHITELIST=internal.service,*.corp.example,172.16.0.0/12,2001:db8::1,fd00::/8
```


## 8. 如何开启和查看 Langfuse 可观测性追踪？

Yuheng 可以把问答、检索、文档入库的完整调用链与各类模型调用的 token 用量上报到 Langfuse（Langfuse Cloud，或用 compose 的 `langfuse` profile 自建）。

在 `.env` 里同时填上 `LANGFUSE_PUBLIC_KEY` 与 `LANGFUSE_SECRET_KEY` 即启用；接 Langfuse Cloud 时不用设 `LANGFUSE_HOST`，自建时设为 `http://langfuse-web:3000`。重启 app 后启动日志出现 `[Langfuse] enabled ...` 即接通，之后在 Langfuse 的 Traces 页查看每次对话与后台任务的调用树与 token 统计。

自建步骤、全部调优变量、token 口径与排查见[可观测性与审计 · 接入 Langfuse](../website-docs/03-features/16-observability.md#langfuse-setup)。

## 9. 什么是 Wiki 模式？如何使用？

Wiki 模式会根据原始文档自动生成并维护一套结构化、相互链接的 Markdown Wiki 页面，把零散资料整理成可以浏览的知识站点。

**使用方法**：
1. **新建知识库时**在「基本信息 → 索引策略」里勾选「Wiki 知识库」。知识库里已经有内容时索引策略被锁定，所以要在上传文档前决定。
2. 按需调整提取粒度、内容生成要求与提取重点，以及「模型配置」里的 Wiki 合成模型。
3. 上传文档后，系统异步用大模型抽取实体与概念，生成页面及页面之间的链接。
4. 在知识库的「Wiki」页签浏览、编辑页面（每次修改都留版本、可回滚），「图谱」页签查看页面之间的链接关系。这张图是 Wiki 页面的引用关系，与需要 Neo4j 的「知识图谱」（实体关系图）不是一回事。

详见 [Wiki 能力](../website-docs/03-features/14-wiki.md)。

## 10. 原本能做的操作提示「权限不足」？

空间内有 RBAC（角色矩阵 + 资源归属），写入接口会按角色与资源创建者鉴权。常见现象：

- **看得到但点不动**：你大概率是该资源的 `Viewer`，或不是创建者的 `Contributor`，界面已经把写操作隐藏或置灰。
- **共享空间里的知识库**：别人共享给你的知识库按共享时授予的权限（只读或可编辑）处理。
- **API Key 调用**：API Key 不沿用成员角色，而是按它被授予的能力（capability）与知识库白名单授权，未声明策略的接口默认拒绝；报 403 时检查这把 Key 是否有对应能力。
- **跨空间超管**：需要 `User.CanAccessAllTenants=true` 且配置 `enable_cross_tenant_access=true`，并通过 `X-Tenant-ID` 切换空间。

需要临时回退到「只记录、不拦截」时，可设置 `tenant.enable_rbac=false`（或环境变量 `YUHENG_TENANT_ENABLE_RBAC=false`）。角色矩阵与 API Key 能力见[租户、用户与认证授权](../website-docs/03-features/01-tenant-auth.md)。

## 11. 为什么登录后没有自动回到上次的工作区？

系统会记住「最后活跃工作区」（`last_active_tenant_id`）并在登录后自动恢复。若仍未恢复，通常是：

1. 浏览器清理了 LocalStorage / 切换了浏览器；
2. 你最后访问的那个工作区已经把你移除（`/leave` 或被管理员剔除）— 系统会回退到默认空间；
3. JWT 中携带了 `tenant_id` 但已无效 — 退出重登录即可。

## 12. 如何让多人协作时正确分配权限？

按照[租户、用户与认证授权](../website-docs/03-features/01-tenant-auth.md)里的角色矩阵：

- 只读用户 → `Viewer`
- 普通成员（上传文档、维护「自己」的 KB）→ `Contributor`
- 运维人员（管理共享模型、向量库、解析器等基础设施）→ `Admin`
- 空间所有者（拥有删除空间权限；每空间至少一位，可以有多位，最后一位不能被降级或移除）→ `Owner`

成员通过空间的「邀请」入口加入。是否允许公网自助注册是平台级设置：系统管理员把 `auth.registration_mode` 设为 `invite_only` 即可关闭公网注册，只能凭邀请加入（见[平台管理与系统管理员](../website-docs/03-features/20-platform-admin.md)）。

## 13. 文档解析卡在「处理中」/ 解析追踪时间线打不开怎么办？

每个文档的解析都会记录一棵阶段树（`knowledge_processing_spans` 表，接口 `GET /api/v1/knowledge/:id/spans`），在文档的「查看 Trace」入口打开侧边时间线，逐阶段查看进度。常见情况：

- **文档长时间停在「处理中」**：先打开时间线看是哪个阶段没有推进（解析 / 切分 / 向量化 / 后处理）。确认某次解析挂死时，可以「停止解析」（`POST /api/v1/knowledge/:id/cancel-parse`）后再重新解析。积压的后台任务也可以在「设置 → 系统管理 → 任务队列」查看。
- **时间线一直显示「更新中」但无数据**：通常是轮询请求失败（网络或反向代理问题），刷新页面，或检查反向代理是否缓冲、截断了响应。

## 14. 能否改用 Elasticsearch / OpenSearch / Milvus / Qdrant 等外部向量库？

只支持一种检索引擎：带 ParadeDB（`pg_search`，BM25）与 pgvector 的 PostgreSQL，即 `RETRIEVE_DRIVER=postgres`。Elasticsearch、OpenSearch、Milvus、Weaviate、Qdrant、Doris、腾讯云 VectorDB 的驱动已经移除，`docker-compose.yml` 也不再带对应服务与环境变量；「设置 → 向量数据库引擎」的注册机制仍在，但没有其他可注册的引擎。详见[检索引擎与向量存储](../website-docs/03-features/05-retrieval-engines.md)。

如果启动时报错，提示 PostgreSQL 缺少 `vector` 或 `pg_search` 扩展，说明所连数据库不是 `docker-compose.yml` 使用的 ParadeDB 镜像。处理办法二选一：

- 直接使用 `docker-compose.yml` 中的 ParadeDB 镜像（已内置两个扩展）；
- 在自有 PostgreSQL 上自行安装 pgvector 与 pg_search。注意云厂商托管的 PostgreSQL 通常无法安装 `pg_search`，此时不能使用托管库。

## 15. 内置模型（builtin models）如何用 YAML 声明式管理？

平台内置模型由 `config/builtin_models.yaml` 声明式驱动，支持 `${ENV}` 变量插值，并通过 `managed_by` 字段与漂移巡检保持数据库与 YAML 一致。常见问题：

- **改了 YAML 不生效**：内置模型在服务启动时做生命周期对账（drift sweep）；确认重启了服务，且条目通过了 schema 校验（ID 长度、必填字段）。
- **Docker 下环境变量未注入**：`builtin_models` 依赖 `env_file` 数组形式注入变量，确认 compose 中按数组形式挂载了 `.env`。
- 参考样例：`config/builtin_models.yaml.example`。
- 系统管理员在界面上保存过某个内置模型后，该行不再由 YAML 托管，之后改 YAML 不会覆盖它；界面也不能删除仍由 YAML 托管的行。

## 16. 系统管理员（System Admin）与平台设置怎么用？

系统管理员管理的是整个部署（全局系统设置、任务队列、平台 API Key、平台审计、集中管控基础设施），与空间内的角色分开。默认的注册模式下，**部署的第一个注册用户自动成为系统管理员**；之后在「设置 → 系统管理 → 系统设置」里提升或撤销其他管理员。第一个用户不合适时，可以在没有任何系统管理员时用 `YUHENG_BOOTSTRAP_SYSTEM_ADMIN_EMAIL` 引导。撤销有防护：不能撤销自己，也不能撤销最后一个系统管理员。详见[平台管理与系统管理员](../website-docs/03-features/20-platform-admin.md)。

## 17. 上传时如何自定义解析配置（process_config）？

文件 / URL / 文件夹上传可携带 `process_config`（`KnowledgeProcessOverrides`），在**本次批次**内覆盖知识库默认的解析引擎、分块、多模态（VLM / ASR）、问题生成、图谱抽取等设置，而不会改动 KB 全局配置。Web UI 在上传前会弹出确认对话框供调整；调 API 时在上传表单里带 `process_config` 字段（JSON 字符串）即可，`yuheng doc upload` 目前没有对应参数。

- **与 KB 默认配置的关系**：未传的字段沿用 KB 默认值；`graph_enabled` 仅在 `extract_config.enabled` 为 true 时生效。
- **重新解析**：`POST /knowledge/:id/reparse` 可在 body 中传 `process_config` 以新配置重跑解析，覆盖项会写入 `knowledge.metadata.process_overrides`。
- **图片 / 音频校验**：批次含图片时需 KB 已配置 VLM；含音频时需已配置 ASR，否则上传会被拒绝。
- 详见 [知识 API](../website-docs/04-api/02-api-knowledge.md)。

## 18. pgvector 检索变慢或刚升级后需要做什么？

迁移 `000059_embeddings_hnsw_1024`，为 **1024 维** embedding（如 bge-m3）在 PostgreSQL pgvector 上创建 HNSW 索引。服务启动会自动执行迁移；若你使用其他维度，该索引可能不适用，需按自身 embedding 维度另行调优。升级后首次大批量入库期间索引构建可能占用额外 I/O，属正常现象。

## 19. 文档如何设置多个标签？

文档标签支持**多标签**（迁移 `000063_knowledge_multi_tags`）。在知识库列表可为文档打多个标签，侧边栏支持按标签筛选；**标签管理**抽屉可批量维护标签。API 上传 / 更新知识时传 `tag_ids` 数组（取代旧的单 `tag_id`）。

## 20. 如何批量重新解析文档？

在知识库文档列表框选多篇文档后，使用批量操作栏的 **重新解析**；也可调用 `POST /knowledge/batch-reparse`，body 可含 `ids` 与可选 `process_config`。任务异步入队，UI 会在入队后刷新状态。单篇仍可用 `POST /knowledge/:id/reparse`。

## 21. RSS 数据源如何配置？

在知识库设置「存储与数据 → 数据源」中选择「RSS / Atom 订阅」，每行填一个订阅源地址，再设同步策略即可全量 / 增量拉取正文入库（私有订阅源可配自定义请求头）。若部分条目失败，同步日志会展示 partial failure 详情；编辑数据源保存配置**不会**自动触发同步，需手动点同步。所有连接器的说明见[数据源导入](../website-docs/03-features/10-datasource.md)。

## 22. Embedding 维度如何覆盖？

在 **设置 → 模型管理** 编辑 Embedding 模型时可填写 **dimensions** 覆盖值（如 1024、1536）。若向量库索引维度与模型不一致，检索可能异常，请保持 KB 绑定向量库与模型维度一致。

## 23. 如何创建并限制权限范围 API Key？

API Key 是独立的机器主体，不等同于某个用户：它要么是全量权限（full access），要么携带显式的能力（capability）集合，并可以限定到指定知识库。

- 空间 Owner 在 **设置 → 空间 → API Key** 创建、修改和吊销（对应 `/api/v1/tenants/:id/api-keys` 下的 GET / POST / PUT / DELETE），可勾选能力（如 `retrieve`、`chat`、`ingest`、`manage_kbs`、`manage_storage_backends`）、限定知识库并设置过期时间。密钥明文只在创建时显示一次。
- 路由按声明的策略放行，未声明策略的接口对 API Key 默认拒绝；给集成用具备所需能力的受限 Key，而不是全量 Key。
- Key 的 `last_used_at` 按节流更新，避免高频写库。

能力清单与知识库白名单见[租户、用户与认证授权](../website-docs/03-features/01-tenant-auth.md)的「API Key 体系」。

### 如何用一个 API Key 自动化管理多个空间？

系统管理员可在 **设置 → 系统管理 → 平台 API Key** 创建 `scope_type=platform` 的 Key。平台 Key 不绑定单一空间：调用普通空间 API 时必须携带 `X-Tenant-ID`，并继续受原有 capability 和知识库范围守卫约束；调用开放的系统控制面接口则需要对应的 `system_*` capability。平台 Key 不支持 `full_access`，也不能创建、轮换或吊销其他平台 Key。

## 24. 一个空间如何绑定多个对象存储实例？

Yuheng 支持**多实例存储后端**（迁移 `000068_storage_backends`）。一个空间可注册多个存储实例（`local` / `s3`，MinIO、RustFS、AWS S3、阿里云 OSS、腾讯云 COS 等都用 `s3` 接入），不同知识库绑定到不同实例，空间维度还有一个默认实例：

- 在 **设置 → 存储引擎** 创建/测试/设为默认（需 Admin+，开启集中管控后创建与修改仅限系统管理员；API Key 需 `manage_storage_backends` 能力）。
- 部署本身的存储（`STORAGE_TYPE`、`S3_*` 配置的那套）是一条共享给所有空间的只读记录 `env`，新空间默认用它。
- 未显式绑定的新知识库与文档空间使用空间默认实例；有文件的知识库也可以改绑，已有文件留在原实例上照常可读，只有新文件写到新实例。
- 响应中的 `access_key_id` / `secret_access_key` 会被掩码，更新时提交掩码占位符不会覆盖库中真实凭据。
- 若创建知识库时提示存储引擎不可用，请确认目标 provider 在 `STORAGE_ALLOW_LIST` 允许范围内。详见[存储后端](../website-docs/03-features/19-storage-backends.md)与[存储后端 API](../website-docs/04-api/02-api-infra.md)。

## 25. 后台解析/入库任务积压或需要排查失败任务怎么办？

系统管理员可使用**运行时任务队列面板**与 **Worker 池治理**。文档处理从单一聚合池改为分阶段独立池（core / 后处理 / enrichment / maintenance）+ 弹性共享池，Wiki 独立治理：

- 在 **设置 → 系统管理 → 任务队列** 查看队列深度、按模型的后台并发、失败任务详情，并可重试、删除或清空归档任务。
- 可通过 `YUHENG_ASYNQ_*_CONCURRENCY` 与 `asynq.*_concurrency` 系统设置调整各池并发（需重启服务）；`model.max_concurrency` 用于约束单模型后台并发。
- 详见 [异步任务系统 · 配置与容量规划](../website-docs/02-architecture/05-async-tasks.md)。注意：Worker 并发只是调度预算，仍受模型配额、DocReader 容量、向量库与数据库连接数限制。

## 26. 对话中如何临时上传图片/文档做一次性问答？

支持**会话级临时附件**（迁移 `000070_temporary_documents`）。在对话输入区上传图片或文档，系统异步解析后仅用于当前会话的问答，不会写入知识库。图片与附件共享一个合并数量上限；附件内容会在多轮对话中保留，解析产物默认 24 小时后清理（`YUHENG_CHAT_ATTACHMENT_TTL_HOURS`）。详见[会话与对话体验](../website-docs/03-features/18-chat-experience.md)。

## 27. 如何为 Redis 启用 TLS？

支持 Redis 的 **TLS 连接**（如 AWS ElastiCache）：设置 `REDIS_USE_TLS=true`；地址是 IP 时用 `REDIS_TLS_SERVER_NAME` 指定证书校验与 SNI 的服务器名；`REDIS_TLS_INSECURE_SKIP_VERIFY=true` 跳过证书校验，只用于开发或自签证书。启动日志会打印 TLS 配置状态便于确认。若连接失败，请核对证书与 Redis 服务端是否要求 TLS。

## 28. 如何使用火山引擎 Rerank / 智谱 AI 网络搜索？

支持两个供应商：

- **火山引擎 Rerank**：在 **设置 → 模型管理** 中添加 Rerank 模型并选择火山引擎。当单次请求文档数超过 API 上限时，客户端会自动分批发送并合并结果。
- **智谱 AI 网络搜索**：在 **设置 → 网络搜索** 中添加智谱搜索并设为默认。设好默认后，Web 对话输入栏会出现网络搜索开关，打开后提问即带 `web_search_enabled: true`；API / SDK / MCP 调用需自己在请求里带这个字段，见[网络搜索与网页抓取](../website-docs/03-features/11-web-search.md)。

## 29. 官方文档在哪里看？如何本地或独立部署文档站？

完整的官方产品文档位于仓库 [`website-docs/`](../website-docs/README.md) 目录，按「入门 → 架构 → 功能 → API → 客户端 → 开发」六个板块组织，覆盖 API 端点、环境变量与扩展点。

该目录同时是一个 VitePress 站点，两种使用方式：

```bash
# 本地预览
cd website-docs && npm install && npm run dev

# 独立容器部署（容器内 Nginx 监听 8081）
docker build -t yuheng-docs website-docs
docker run -d -p 8081:8081 yuheng-docs
```

站点的版本号在构建时自动读取仓库根目录的 `VERSION` 文件，因此升级版本后无需手动改文档。

`website-docs/sample-data/` 下还提供了 4 份 Markdown 样例文档与 1 份 FAQ 导入 JSON，可以直接用来跑一遍「建库 → 上传 → 问答」。

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
- `public` 会为每个被引用文件签发**限时匿名可读**链接（Yuheng 的 `/r/<token>` 2 小时，S3 预签名 24 小时），请评估是否符合你的安全要求。
- 限定了知识库范围的 API Key 在 `public` 模式下（无论来自请求参数还是 `RESOURCE_URL_MODE`）会返回 **403**，这类 Key 需要带 `?resource_urls=handle`。
- 建议同时配置 `SYSTEM_AES_KEY`，以便复用 grant 行、稳定直链 URL 并降低读接口的写入压力。

详见 [图片与文件的对外访问](../website-docs/03-features/21-file-access.md)。

## 34. 使用 AWS S3 但不想在配置里写 AK/SK？

支持 **AWS SDK 默认凭据链**：把 `S3_ACCESS_KEY` 与 `S3_SECRET_KEY` **同时留空**即可，SDK 会依次尝试 EC2/ECS/EKS 实例角色、IRSA / Web Identity、环境变量与共享配置文件。注意两者必须同时填写或同时留空，只填一个会报配置错误。`S3_ENDPOINT` 也可留空，此时使用 `S3_REGION` 对应的 AWS 标准端点。

## 35. MCP Server 用 `uvx` 启动失败，或者应该装哪个包？

Yuheng 的 MCP Server 包名为 **`yuheng-mcp`**，命令行入口是 `yuheng-mcp-server` / `yuheng-server`（源码运行：`uv run --project mcp-server yuheng-mcp-server`）。

该包没有发布到 PyPI，`uvx --from yuheng-mcp ...` 这类从 PyPI 拉包的写法会失败，请从源码安装（`pip install ./mcp-server`）或用上面的 `uv run`。工具共 23 个（租户 / 知识库 / 知识 / 检索 / RAG 问答 / 分块 / Wiki / 模型管理），传输支持 stdio / SSE / HTTP，网络传输必须配置 `MCP_SERVER_AUTH_TOKEN`。只需要只读访问时，也可以用 `yuheng mcp serve`（8 个只读工具）。详见 [MCP 集成](../website-docs/03-features/08-mcp.md)与 [`mcp-server/MCP_CONFIG.md`](../mcp-server/MCP_CONFIG.md)。

行为变化提醒：工具执行失败时，MCPServer 2.x 返回 `CallToolResult(isError=True)`，不再像旧版低层 API 那样以成功响应返回 `"Error executing …"` 文本前缀。只解析 `content[0].text` 的客户端通常无感，依赖 `isError` 标志的集成方行为会更符合 MCP 规范。

## P.S.
如果以上方式未解决问题，请提交 issue 描述问题，并附上相关日志（`docker compose logs app`，必要时把 `LOG_LEVEL` 设为 `debug`）。
