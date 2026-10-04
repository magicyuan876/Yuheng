# API 参考：基础设施与数据源

路由注册：`internal/router/routes_infra.go` 的 `RegisterVectorStoreRoutes`、`RegisterStorageBackendRoutes`、`RegisterWebSearchRoutes`、`RegisterWebSearchProviderRoutes`、`RegisterDataSourceRoutes`。Handler：`internal/handler/vectorstore.go`、`internal/handler/storagebackend.go`、`internal/handler/web_search.go`、`internal/handler/web_search_provider.go`、`internal/handler/web_search_provider_credentials.go`、`internal/handler/datasource.go`、`internal/handler/datasource_credentials.go`。

统一约定：读 Viewer+。向量存储、存储后端、Web 搜索提供方的写操作与连接测试挂 `PlatformManaged` 守卫（`internal/router/rbac.go`）：系统设置 `governance.centralized_infra` 关闭时为 Admin+，开启后只允许系统管理员；读接口始终 Viewer+，便于在知识库编辑页选用平台资源。数据源的写操作为 Admin+。文中写作“权限：平台管理”的即指 `PlatformManaged`。API key capability：向量库 `manage_vector_stores`、存储后端 `manage_storage_backends`、Web 搜索 `manage_web_search`、数据源 `manage_datasources`（均可 full-access）。

## 向量存储（/api/v1/vector-stores）

检索引擎只有 PostgreSQL 一种（ParadeDB 做关键词检索、pgvector 做向量检索），跑在应用自己的数据库里，由环境变量 `RETRIEVE_DRIVER=postgres` 启用，以虚拟 store `__env_postgres__` 的形式出现在列表中。该引擎不可注册（`internal/application/service/retriever/engine_postgres.go`：嵌入表名固定、不按 store 分区，同一台服务器上的第二个 store 隔离不了任何数据），所以当前版本 `GET /types` 返回空数组，创建和原始配置测试会被拒绝。下面的注册类接口为引擎注册表（`internal/application/service/retriever/catalog.go`）预留，供将来可注册的引擎使用。

本组错误响应是 `{"success":false,"error":"..."}`（字符串，不是 `AppError` 结构）；修改或删除 env store 返回 400 `environment-configured vector stores cannot be modified via API`。

### GET /api/v1/vector-stores/types

用途：可注册的引擎类型与表单字段（`type,display_name,connection_fields,index_fields`）。权限：Viewer+。

响应：200 `{"success":true,"data":[]}`（当前版本没有可注册的引擎）

```bash
curl $BASE/api/v1/vector-stores/types -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/vector-stores

用途：向量库列表（环境变量注入的 `__env_*` store 在前）。权限：Viewer+。

响应：200 `{"success":true,"data":[VectorStoreResponse]}`（`id,tenant_id,name,engine_type,connection_config,index_config,is_builtin,source,readonly,created_at,updated_at`；连接配置中的凭证掩码）。`source` 为 `env` 或 `user`，env store 的 `readonly` 为 true。PostgreSQL 的 env store 形如 `{"id":"__env_postgres__","name":"PostgreSQL","engine_type":"postgres","connection_config":{"use_default_connection":true},"source":"env","readonly":true}`。平台共享的 store 对非平台管理员整段隐去 `connection_config`，只保留名称、引擎类型与状态。

```bash
curl $BASE/api/v1/vector-stores -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/vector-stores/:id

用途：向量库详情（支持 `__env_*` ID）。权限：Viewer+。

响应：200 `{"success":true,"data":{VectorStoreResponse}}`

```bash
curl $BASE/api/v1/vector-stores/__env_postgres__ -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/vector-stores/:id/test

用途：测试已保存或 env 向量库的连通性。权限：平台管理。PostgreSQL 引擎就在应用自己的数据库里，只要应用在运行即视为可达。

响应：200 `{"success":true,"version":"..."}`；测试失败时 HTTP 状态码仍是 200，返回 `{"success":false,"error":"..."}`。引擎报不出版本时 `version` 为空串（PostgreSQL 即如此）。

```bash
curl -X POST $BASE/api/v1/vector-stores/__env_postgres__/test -H "Authorization: Bearer $TOKEN"
```

### 注册类接口

以下接口只对可注册的引擎生效，当前版本调用创建或原始测试会返回错误。权限均为平台管理。

| 方法与路径 | 用途 | 请求体 |
| --- | --- | --- |
| `POST /api/v1/vector-stores/test` | 用原始配置测试连接（不落库） | `engine_type`、`connection_config`（均必填） |
| `POST /api/v1/vector-stores` | 创建向量库配置，201 | `name`、`engine_type`、`connection_config`（必填），`index_config`（可选） |
| `PUT /api/v1/vector-stores/:id` | 仅重命名；env store 不可改 | `{"name":"..."}`（必填） |
| `DELETE /api/v1/vector-stores/:id` | 删除；env store 不可删 | 无 |
| `PUT /api/v1/vector-stores/:id/sharing` | 设为平台共享或取消共享（服务层限定系统管理员） | `{"shared":true}`（必填） |

## 存储后端（/api/v1/storage-backends）

请求体（Create/Update/TestRaw 共用 `storageBackendRequest`）：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是（`binding:"required"`） | 名称 |
| `provider` | string | 是（`binding:"required"`） | 提供方（`local` 或 `s3`，S3 协议兼容的对象存储都走 `s3`；S3 兼容服务的 endpoint 与 `addressing_style` 见[安装部署](../01-getting-started/02-installation.md)） |
| `config` | object | 否 | 提供方配置，见下表（响应中凭证掩码） |
| `status` | string | 否 | `active`（默认）或 `disabled` |

`config` 字段：`local` 只看 `path_prefix`，其余字段属于 `s3`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `endpoint` | string | S3 兼容服务地址，可带 `http://` / `https://`；不带协议时按 `use_ssl` 补；留空表示 AWS S3 |
| `region` | string | 区域，`s3` 必填 |
| `bucket_name` | string | Bucket，`s3` 必填；不存在时首次使用自动创建 |
| `access_key_id` / `secret_access_key` | string | 必须同时提供或同时留空，同时留空走 AWS 默认凭证链；加密存储，响应中为 `***` |
| `path_prefix` | string | 对象前缀，必须是相对路径，不能以 `/` 开头，也不能用 `..` 上跳 |
| `use_ssl` | bool | 只在 `endpoint` 不带协议头时生效 |
| `addressing_style` | string | `auto`（默认）/ `path` / `virtual`。`auto` 时 endpoint 为空或属于 `amazonaws.com` 用 virtual-hosted，其他 endpoint（RustFS、MinIO 等）用 path-style；阿里云 OSS、腾讯云 COS、火山引擎 TOS、华为云 OBS 不接受 path-style，必须显式设为 `virtual` |

响应里的 `StorageBackend` 另有 `id`、`tenant_id`、`source`、`is_builtin`（平台共享）、`created_at`、`updated_at`。`source` 为 `user` 是工作区注册的实例；`env` 是部署存储，全平台只有一条（`id` 为 `env`，`tenant_id` 为 0），由 `STORAGE_TYPE`、`S3_*` 等环境变量在每次启动时写入，不存凭证，共享给所有工作区，不能经 API 修改、删除或取消共享。

规则一览（违反时返回 400，名称冲突 409）：

- 创建与更新都会先校验配置、对 `s3` 的 endpoint 做 SSRF 校验（`rustfs:9000` 这类内网地址要在 `SSRF_WHITELIST` 中放行），再实际连一次；连接失败的错误信息经过脱敏，不含内部主机名、IP、端口与 TLS 细节。同一工作区内名称唯一。
- `provider` 与决定物理位置的 `endpoint`、`region`、`bucket_name`、`path_prefix` 创建后不可改，要换位置走存储迁移；凭证可以单独轮换：更新时凭证字段传 `***` 表示保留原值，传空字符串表示清除（两把都清除即改用 AWS 默认凭证链）。
- 停用、删除、取消共享按**所有工作区**统计仍在使用它的地方：设为默认的工作区、绑定它的知识库与文档空间、存在它上面的有效文件。停用与删除在有任何一项时拒绝；取消共享只看所属工作区以外的使用。删除是软删除，平台共享中的实例须先取消共享。
- 工作区能看到的任何 `active` 实例都能设为默认：自己的、平台共享的、部署存储 `env`。

### GET /api/v1/storage-backends/types

用途：允许的存储类型：`local`、`s3` 中被环境变量 `STORAGE_ALLOW_LIST` 放行的部分（未设置时两者都放行）。权限：Viewer+。响应：200 `{"success":true,"data":["local","s3"]}`

```bash
curl $BASE/api/v1/storage-backends/types -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/storage-backends/test

用途：原始配置连接测试，不落库。权限：平台管理。响应：200 `{"success":true}`；失败时仍为 200，返回 `{"success":false,"error":"<脱敏后的原因>"}`。

```bash
curl -X POST $BASE/api/v1/storage-backends/test -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"t","provider":"s3","config":{"endpoint":"http://rustfs:9000","region":"us-east-1","bucket_name":"yuheng","access_key_id":"rustfsadmin","secret_access_key":"rustfsadmin","addressing_style":"path"}}'
```

### POST /api/v1/storage-backends

用途：创建存储后端。权限：平台管理。详见[存储后端](../03-features/19-storage-backends.md)。响应：201 `{"success":true,"data":{StorageBackend}}`

```bash
curl -X POST $BASE/api/v1/storage-backends -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"rustfs-main","provider":"s3","config":{"endpoint":"http://rustfs:9000","region":"us-east-1","bucket_name":"yuheng","access_key_id":"rustfsadmin","secret_access_key":"rustfsadmin","addressing_style":"path"}}'
```

### GET /api/v1/storage-backends

用途：列表（含 `default_storage_backend_id`）。权限：Viewer+。响应：200 `{"success":true,"data":[...],"default_storage_backend_id":"..."}`

```bash
curl $BASE/api/v1/storage-backends -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/storage-backends/:id

用途：详情（凭证掩码）。权限：Viewer+。响应：200 `{"success":true,"data":{StorageBackend}}`

```bash
curl $BASE/api/v1/storage-backends/sb-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/storage-backends/:id

用途：更新。权限：平台管理。响应：200 `{"success":true,"data":{StorageBackend}}`

```bash
curl -X PUT $BASE/api/v1/storage-backends/sb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"s3-prod","provider":"s3"}'
```

### DELETE /api/v1/storage-backends/:id

用途：删除。权限：平台管理。响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/storage-backends/sb-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/storage-backends/:id/test

用途：用已保存的凭证测试连通性。权限：平台管理。响应：同原始配置测试，失败也返回 200。

```bash
curl -X POST $BASE/api/v1/storage-backends/sb-1/test -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/storage-backends/:id/sharing

用途：设为平台共享（所有工作区可见可选用，端点与凭证对非系统管理员隐藏）或取消共享。权限：平台管理，服务层限定系统管理员。请求体：`{"shared":true}`（必填）。取消共享时，若 owner 之外的工作区仍有默认存储、知识库或活跃资源绑定，返回 400。

响应：200 `{"success":true,"data":{StorageBackend}}`

```bash
curl -X PUT $BASE/api/v1/storage-backends/sb-1/sharing -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"shared":true}'
```

### PUT /api/v1/storage-backends/:id/default

用途：设为本工作区的默认后端。权限：Admin+（集中管理模式下也是 Admin+：它只是在可用后端中选一个给本工作区用，不改后端本身）。响应：200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/storage-backends/sb-1/default -H "Authorization: Bearer $TOKEN"
```

## Web 搜索（/api/v1/web-search 与 /api/v1/web-search-providers）

### GET /api/v1/web-search/providers

用途：内置搜索提供方目录（只读）。权限：Viewer+，仅 JWT（未声明 API key 策略）。Handler: `internal/handler/web_search.go`

响应：200 `{"success":true,"data":[...]}`

```bash
curl $BASE/api/v1/web-search/providers -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/web-search-providers/types

用途：提供方类型与参数 schema，供前端动态生成表单。权限：Viewer+。Handler: `internal/handler/web_search_provider.go`

响应：200 `{"success":true,"data":[{id,name,description,docs_url,requires_api_key,supports_optional_api_key,requires_engine_id,requires_base_url,supports_proxy,config_fields}]}`。`config_fields` 描述该提供方的非密钥配置项（`key,label,type,required,default,options,...`），取值保存在 `parameters.extra_config`。`GET /web-search/providers` 返回的是同一份目录。

```bash
curl $BASE/api/v1/web-search-providers/types -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/web-search-providers/test

用途：原始凭证测试（不落库）。权限：平台管理。请求体：`provider`（`binding:"required"`）、`parameters`（可选）。

响应：200 `{"success":bool,"error"}`

```bash
curl -X POST $BASE/api/v1/web-search-providers/test -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"provider":"tavily","parameters":{"api_key":"tvly-..."}}'
```

### POST /api/v1/web-search-providers

用途：创建提供方配置。权限：平台管理。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是（`binding:"required"`） | 名称 |
| `provider` | string | 是（`binding:"required"`） | 类型：`bing`、`google`、`duckduckgo`、`tavily`、`ollama`、`baidu`、`searxng`、`keenable`、`zhipu`、`exa`、`metaso`、`firecrawl` |
| `description` | string | 否 | 描述 |
| `parameters` | object | 否 | `api_key`（建议走 credentials 子资源）、`engine_id`（如 Google CSE）、`base_url`（如自建 SearXNG）、`proxy_url`（`supports_proxy` 的提供方才生效）、`extra_config`（map） |
| `is_default` | bool | 否 | 默认提供方 |

响应：201 `{"success":true,"data":{WebSearchProviderResponse}}`

智谱的 `extra_config`：`search_engine` 取 `search_std`（默认）、`search_pro`、`search_pro_sogou`、`search_pro_quark`；`content_size` 取 `medium`（默认）或 `high`，非法值会被拒绝；智谱必须带 `api_key`。

```bash
curl -X POST $BASE/api/v1/web-search-providers -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"tavily-main","provider":"tavily"}'

curl -X POST $BASE/api/v1/web-search-providers -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"zhipu","provider":"zhipu","parameters":{"api_key":"<zhipu-key>","extra_config":{"search_engine":"search_std","content_size":"medium"}}}'
```

### GET /api/v1/web-search-providers

用途：提供方列表。权限：Viewer+。响应：200 `{"success":true,"data":[...]}`

```bash
curl $BASE/api/v1/web-search-providers -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/web-search-providers/:id

用途：详情。权限：Viewer+。响应：200 `{"success":true,"data":{...}}`

```bash
curl $BASE/api/v1/web-search-providers/wsp-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/web-search-providers/:id

用途：更新（空字段保留原值；APIKey 保留）。权限：平台管理。请求体：`name/description/parameters/is_default`（均可选）。提供方类型 `provider` 创建后不能改，请求里没有这个字段。

响应：200 `{"success":true,"data":{...}}`

```bash
curl -X PUT $BASE/api/v1/web-search-providers/wsp-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"is_default":true}'
```

### DELETE /api/v1/web-search-providers/:id

用途：删除。权限：平台管理。响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/web-search-providers/wsp-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/web-search-providers/:id/credentials

用途：设置 API key（`{"api_key":"..."}`，省略时返回状态）。权限：平台管理。Handler: `internal/handler/web_search_provider_credentials.go`

响应：200 `{"success":true,"data":{"fields":{"api_key":{"configured":bool}}}}`

```bash
curl -X PUT $BASE/api/v1/web-search-providers/wsp-1/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"api_key":"tvly-..."}'
```

### DELETE /api/v1/web-search-providers/:id/credentials/:field

用途：删除凭证字段（`field` 仅 `api_key`）。权限：平台管理。响应：204。

```bash
curl -X DELETE $BASE/api/v1/web-search-providers/wsp-1/credentials/api_key -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/web-search-providers/:id/test

用途：测试已保存提供方。权限：平台管理。响应：200 `{"success":bool,"error"}`

```bash
curl -X POST $BASE/api/v1/web-search-providers/wsp-1/test -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/web-search-providers/:id/sharing

用途：设为平台共享或取消共享。权限：平台管理，服务层限定系统管理员。请求体：`{"shared":true}`（必填）。

响应：200 `{"success":true,"data":{...}}`

```bash
curl -X PUT $BASE/api/v1/web-search-providers/wsp-1/sharing -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"shared":false}'
```

## 数据源（/api/v1/datasource）

外部内容连接器（`internal/datasource/`：飞书、Notion、语雀、GitLab、IMA、RSS），同步任务会写入 KB，详见[数据源导入](../03-features/10-datasource.md)。Handler: `internal/handler/datasource.go`。本组多数响应为原始对象/数组（无 `success` 包装）。

### GET /api/v1/datasource/types

用途：可用连接器目录。权限：Viewer+。

响应：200 `[{type,name,description,icon,priority,auth_type,capabilities}]`

```bash
curl $BASE/api/v1/datasource/types -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/datasource/validate-credentials

用途：校验原始凭证（“测试连接”按钮，不落库）。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `type` | string | 是（`binding:"required"`） | 连接器类型 |
| `credentials` | map | 是（`binding:"required"`） | 凭证 |

响应：200 `{"status":"connected"}`；失败 400 `{"error":"..."}`

```bash
curl -X POST $BASE/api/v1/datasource/validate-credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"type":"notion","credentials":{"token":"secret"}}'
```

### POST /api/v1/datasource

用途：创建数据源。权限：Admin+。请求体（`types.DataSource`）：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `knowledge_base_id` | string | 是 | 目标 KB（须归属本工作区） |
| `name` | string | 是 | 名称 |
| `type` | string | 是 | 连接器类型 |
| `config` | object | 是 | 凭证（加密存储）+资源选择+设置 |
| `sync_schedule` | string | 否 | cron 表达式 |
| `sync_mode` | string | 否 | `incremental`（默认）/`full` |
| `conflict_strategy` | string | 否 | `overwrite`（默认）/`skip` |
| `sync_deletions` | bool | 否 | 默认 true |
| `sync_log_retention_days` | int | 否 | 默认 30 |

响应：201 `DataSourceResponse`（凭证剥离，见 `internal/handler/dto/datasource.go`）。

```bash
curl -X POST $BASE/api/v1/datasource -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"knowledge_base_id":"kb-1","name":"notion 同步","type":"notion","config":{}}'
```

### GET /api/v1/datasource

用途：数据源列表。权限：Viewer+。查询参数：`kb_id`（必填）。

响应：200 `[DataSourceResponse]`

```bash
curl "$BASE/api/v1/datasource?kb_id=kb-1" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/:id

用途：详情。权限：Viewer+。响应：200 `DataSourceResponse`；404 `{"error":"data source not found"}`

```bash
curl $BASE/api/v1/datasource/ds-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/datasource/:id

用途：更新（`id/tenant_id/knowledge_base_id` 锁定为原值）。权限：Admin+。请求体同创建。

响应：200 `DataSourceResponse`

```bash
curl -X PUT $BASE/api/v1/datasource/ds-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"notion 同步 v2","type":"notion","knowledge_base_id":"kb-1","config":{}}'
```

### DELETE /api/v1/datasource/:id

用途：删除。权限：Admin+。响应：204。

```bash
curl -X DELETE $BASE/api/v1/datasource/ds-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/datasource/:id/credentials

用途：整体替换凭证（数据源凭证为“单一逻辑字段 `credentials`”的原子 map）。权限：Admin+。请求体：`{"credentials":{...}}`（非空 map 必填）。Handler: `internal/handler/datasource_credentials.go`

响应：200 `{"success":true,"data":{"fields":{"credentials":{"configured":bool}}}}`

```bash
curl -X PUT $BASE/api/v1/datasource/ds-1/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"credentials":{"token":"secret"}}'
```

### DELETE /api/v1/datasource/:id/credentials/:field

用途：清空凭证（`field` 必须为 `credentials`）。权限：Admin+。响应：204。

```bash
curl -X DELETE $BASE/api/v1/datasource/ds-1/credentials/credentials -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/datasource/:id/validate

用途：校验已保存数据源连接。权限：Admin+。响应：200 `{"status":"connected"}`

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/validate -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/:id/resources

用途：浏览外部资源树（懒加载）。权限：Admin+。查询参数：`parent_id`（可选，空=顶层）。

响应：200 `[{external_id,name,type,description,url,modified_at,parent_id,has_children,metadata}]`

```bash
curl "$BASE/api/v1/datasource/ds-1/resources?parent_id=" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/datasource/:id/resource-ancestors

用途：解析资源祖先链（选择器展开）。权限：Admin+。请求体：`{"resource_ids":["..."]}`（必填）。

响应：200 `{"ancestors":[...]}`

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/resource-ancestors -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"resource_ids":["page-1"]}'
```

### POST /api/v1/datasource/:id/sync

用途：手动触发同步。权限：Admin+。响应：200 `SyncLog`（`id,status,started_at,items_total,items_created,items_updated,items_deleted,items_failed,...`）

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/sync -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/datasource/:id/pause 与 POST /api/v1/datasource/:id/resume

用途：暂停 / 恢复定时同步。权限：Admin+。

响应：200 `{"status":"paused"}` / `{"status":"active"}`

```bash
curl -X POST $BASE/api/v1/datasource/ds-1/pause -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/:id/logs

用途：同步日志列表。权限：Viewer+。查询参数：`limit`（默认 10，上限 100）、`offset`（默认 0）。

响应：200 `[SyncLog]`

```bash
curl "$BASE/api/v1/datasource/ds-1/logs?limit=10" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/datasource/logs/:log_id

用途：单条同步日志。权限：Viewer+。响应：200 `SyncLog`；404 `{"error":"sync log not found"}`

```bash
curl $BASE/api/v1/datasource/logs/log-1 -H "Authorization: Bearer $TOKEN"
```
