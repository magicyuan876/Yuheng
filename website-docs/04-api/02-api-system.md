# API 参考：系统与平台管理

这一组是部署级接口：读系统信息与部署能力，以及系统管理员专属的平台控制面（全局设置、平台级解析引擎配置、运行时队列、平台 API Key、跨空间审计、创建用户与重置密码）。功能说明见[平台管理与系统管理员](../03-features/20-platform-admin.md)。

路由注册：`internal/router/routes_auth_tenant.go` 的 `RegisterSystemAdminRoutes` 与 `RegisterSystemRoutes`。Handler：`internal/handler/system.go`、`internal/handler/audit_log.go`。

`/system/admin/*` 全组挂 `SystemAdmin()` 守卫；平台 API Key 按能力细分（`system_settings_read/manage`、`system_runtime_read/manage`、`system_tenants_read/manage`、`system_audit_read`）。只有设置、运行时队列、批量配额与审计日志对平台 key 开放；管理员授予与撤销、创建用户、重置密码、平台 API key 管理、平台解析引擎配置只接受 JWT。

## 系统信息（/api/v1/system）

Handler: `internal/handler/system.go`、`internal/handler/deployment_capabilities.go`。API key：`capabilities`、`upload-limits`、`governance` 三个只读接口任何有效 key 都可调用，其余需 `manage_vector_stores` 或 full-access。本组响应使用 `{"code":0,"msg":"success","data":...}` 包装。

### GET /api/v1/system/capabilities

用途：部署能力清单：当前部署实际注册了哪些功能路由，前端据此隐藏不存在的入口。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":{"capabilities":{"<key>":{"supported":bool,"reason":"..."}},"docs_collab_url":"...","extensions":{...}}}`

- `capabilities` 的键固定为 `organizations`、`settings.websearch`、`settings.vectorstore`、`settings.storage`、`docs`、`docs.public_sharing`；只有 `supported:false` 表示入口应隐藏，此时 `reason` 为 `route_not_registered`。
- `docs_collab_url` 是在线文档协同服务的浏览器侧 WebSocket 地址，未启用在线文档或未配置协同服务时省略（编辑器退回租约独占编辑）。
- `extensions` 列出扩展注册的功能；与 `capabilities` 不同，缺少某个键就表示该功能不存在。没有安装扩展时省略。

```bash
curl $BASE/api/v1/system/capabilities -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/upload-limits

用途：当前生效的上传大小上限（MB），来自系统设置 `file.max_size_mb` / `file.video_max_size_mb`，系统管理员修改后下一次调用即生效。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":{"max_file_size_mb":N,"max_video_file_size_mb":N}}`

```bash
curl $BASE/api/v1/system/upload-limits -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/governance

用途：是否开启「集中管控基础设施」（系统设置 `governance.centralized_infra`）。开启后模型、Web 搜索、解析引擎、向量存储、存储后端等共享基础设施的写操作只允许系统管理员（见[总览](./01-api-overview.md)的 PlatformManaged），前端据此隐藏空间管理员的设置入口。权限：Viewer+；返回值只有一个布尔量，不含配置内容。

响应：200 `{"code":0,"msg":"success","data":{"centralized_infra":bool}}`

```bash
curl $BASE/api/v1/system/governance -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/info

用途：系统版本与引擎信息。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":{version,commit_id,build_time,go_version,keyword_index_engine,vector_store_engine,graph_database_engine,db_version,db_migration_error,started_at,uptime_seconds}}`。`db_version` 在迁移失败时带 `(failed)` 或 `(dirty)` 后缀，`db_migration_error` 给出最近一次启动迁移的错误信息。

```bash
curl $BASE/api/v1/system/info -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/parser-engines

用途：解析引擎列表与 DocReader 连接状态。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":[...],"docreader_addr","docreader_transport","connected"}`

```bash
curl $BASE/api/v1/system/parser-engines -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/parser-engines/check

用途：用给定配置探测解析引擎（`types.ParserEngineConfig` 请求体）。权限：PlatformManaged（集中管控关闭时 Admin+，开启后仅 SystemAdmin）。

响应：200，同上。

```bash
curl -X POST $BASE/api/v1/system/parser-engines/check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{}'
```

### POST /api/v1/system/docreader/reconnect

用途：重连 DocReader。权限：PlatformManaged。请求体：`{"addr":"host:port"}`（`binding:"required"`）。

响应：200 `{"code":0,"msg":"连接成功",...,"connected":true}`

```bash
curl -X POST $BASE/api/v1/system/docreader/reconnect -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"addr":"docreader:50051"}'
```

### GET /api/v1/system/storage-engine-status

用途：对象存储引擎可用性。权限：Viewer+。

响应：200 `{"code":0,"msg":"success","data":{"engines":[{name,allowed,available,description}],"allowed_providers":[...]}}`（`engines` 只列出 `local` 与 `s3`）

```bash
curl $BASE/api/v1/system/storage-engine-status -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/storage-engine-check

用途：校验存储配置（SSRF 防护后探测）。权限：PlatformManaged。请求体：`provider`（必填，`local` 或 `s3`）+ `s3` 配置对象（`endpoint`、`region`、`access_key_id`、`secret_access_key`、`bucket_name`、`path_prefix`、`use_ssl`、`addressing_style`）。

响应：200 `{"code":0,"data":{"ok","message"}}`

```bash
curl -X POST $BASE/api/v1/system/storage-engine-check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"provider":"s3","s3":{"endpoint":"http://rustfs:9000","region":"us-east-1","bucket_name":"yuheng","access_key_id":"rustfsadmin","secret_access_key":"rustfsadmin","addressing_style":"path"}}'
```

## 系统管理（/api/v1/system/admin，SystemAdmin 专属）

组级挂载 `SystemAdmin()` 守卫（始终强制，不受 EnableRBAC 影响）；平台 API key 需对应 `system_*` capability。本组读取接口多返回原始行/数组（无包装）。Handler: `internal/handler/system.go`、`internal/handler/audit_log.go`。

### POST /api/v1/system/admin/promote

用途：授予 SystemAdmin。请求体：`user_id`（UUID，优先）或 `email`（二选一）。

响应：200 `UserInfo`（原始对象）。

```bash
curl -X POST $BASE/api/v1/system/admin/promote -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"admin@ex.com"}'
```

### POST /api/v1/system/admin/revoke

用途：撤销 SystemAdmin。请求体：`{"user_id":"..."}`（`binding:"required"`）。

响应：200 `UserInfo`

```bash
curl -X POST $BASE/api/v1/system/admin/revoke -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"user_id":"u-1"}'
```

### GET /api/v1/system/admin/list

用途：SystemAdmin 列表。查询参数：`offset`（默认 0）、`limit`（默认 50，上限 200）。

响应：200 `{"total":N,"admins":[UserInfo]}`

```bash
curl $BASE/api/v1/system/admin/list -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/users/reset-password

用途：重置另一个用户的本地密码，并吊销其全部现有会话。请求体：`email`（`binding:"required,email"`）、`new_password`（`binding:"required"`，须满足密码策略：8-32 个字符，至少含一个字母和一个数字）。不能用这个接口重置自己的密码（400），自己改密码走 `POST /auth/change-password`。

响应：200 `{"message":"Password reset successfully"}`；用户不存在 404。

```bash
curl -X POST $BASE/api/v1/system/admin/users/reset-password -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"a@ex.com","new_password":"newpass1"}'
```

### POST /api/v1/system/admin/users/create

用途：由系统管理员创建本地用户。是否同时开个人空间，按系统设置 `auth.default_tenant_mode` 的统一策略处理。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `username` | string | 是（`binding:"required,min=2,max=50"`） | 用户名，去掉首尾空白后仍需 2-50 字符 |
| `email` | string | 是（`binding:"required,email"`） | 邮箱 |
| `password` | *string | 否 | 省略或为 null 时服务端随机生成；只要提供了值（包括空串）就按密码策略校验 |

响应：201 `{"user":{UserInfo},"generated_password":"..."}`，`generated_password` 只在服务端生成密码时出现，且只返回这一次，不写日志和审计。邮箱或用户名已存在且指向同一用户时幂等返回 200 与已有用户；邮箱与用户名分属不同用户返回 409。

```bash
curl -X POST $BASE/api/v1/system/admin/users/create -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"username":"bob","email":"bob@ex.com"}'
```

### GET /api/v1/system/admin/api-keys

用途：平台 API key 列表（掩码）。

响应：200 `{"success":true,"data":[{id,name,api_key,capabilities,expires_at_unix,...}]}`

```bash
curl $BASE/api/v1/system/admin/api-keys -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/api-keys

用途：创建平台 API key（明文仅在响应的 `data.token` 里返回一次，之后列表只有掩码）。请求体：`name`（非空）、`capabilities`（必填，每一项都必须是已知 capability，否则 1010；可以是 `system_*`，也可以是作用于 `X-Tenant-ID` 所选空间的空间 capability）、`expires_at_unix`（可选，须为未来时间）。平台 key 没有 `full_access`，也不能创建或删除别的平台 key（这三个管理接口只接受 JWT）。平台 key 的调用方式见[总览](./01-api-overview.md)。

响应：201 `{"success":true,"data":{...,"api_key":"<明文>","token":"<明文>"}}`

```bash
curl -X POST $BASE/api/v1/system/admin/api-keys -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"ops","capabilities":["system_tenants_read"]}'
```

### DELETE /api/v1/system/admin/api-keys/:key_id

用途：删除平台 API key。

响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/api-keys/3 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/settings 与 GET /api/v1/system/admin/settings/:key

用途：平台运行时设置列表 / 单项（平台 key 需 `system_settings_read|manage`）。

响应：200 `[SystemSetting]` / `SystemSetting`（原始，无包装；字段：`key,value,value_type,description,last_modified_by,last_modified_at`）。

```bash
curl $BASE/api/v1/system/admin/settings -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/system/admin/settings/:key

用途：更新设置（平台 key 需 `system_settings_manage`）。请求体：`{"value":<任意 JSON，按注册表类型校验>}`（必填）。

响应：200 `SystemSetting`

```bash
curl -X PUT $BASE/api/v1/system/admin/settings/default_storage_quota -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"value":10737418240}'
```

### DELETE /api/v1/system/admin/settings/:key

用途：删除该键的数据库覆盖值，回落到环境变量（如有）或代码默认值；幂等，从未写过的键也返回 200。平台 key 需 `system_settings_manage`。

响应：200 `{"success":true}`；未知键 400。

```bash
curl -X DELETE $BASE/api/v1/system/admin/settings/default_storage_quota -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/parser-engine-config 与 PUT 同路径

用途：平台级解析引擎配置，处于「部署 ENV < 平台默认 < 空间覆盖」的中间层；空间层仍走 `PUT /tenants/kv/parser-engine-config`（开启集中管控后空间不能再覆盖）。仅 JWT 的 SystemAdmin（未对平台 API key 声明）。

- GET 响应：200 `{"success":true,"data":{ParserEngineConfig}}`，凭据字段以掩码返回，即使调用者是系统管理员。
- PUT 请求体：`types.ParserEngineConfig`；凭据字段原样传回掩码占位符表示不修改。出站地址要通过 SSRF 校验，不通过返回 1010。响应：200 `{"success":true,"data":{...},"message":"..."}`。

```bash
curl $BASE/api/v1/system/admin/parser-engine-config -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/runtime/queues

用途：asynq 队列深度与并发状态（无 Redis 时返回 `available:false`；平台 key 需 `system_runtime_read|manage`）。

响应：200 `{"available",upstream_concurrency,parse_concurrency,wiki_concurrency,pools,queues,model_limiter_available,models,timestamp}`

```bash
curl $BASE/api/v1/system/admin/runtime/queues -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/runtime/queues/:queue/tasks

用途：队列任务列表。查询参数：`state`（`pending/active/scheduled/retry/archived/completed`）、`cursor`、`page_size`（默认 20，上限 100）。

响应：200 `{"available","tasks":[RuntimeTaskInfo],"page_size","has_more","next_cursor"}`

```bash
curl "$BASE/api/v1/system/admin/runtime/queues/default/tasks?state=pending" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/runtime/queues/:queue/tasks/:task_id/actions/:action

用途：任务操作（`action` ∈ `cancel/run_now/delete`；平台 key 需 `system_runtime_manage`）。

响应：200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/system/admin/runtime/queues/default/tasks/t-1/actions/cancel \
  -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/system/admin/runtime/queues/:queue/archived

用途：清空某个队列的全部归档（最终失败）任务，不影响 pending/active/scheduled/retry 任务，也不回写业务状态。平台 key 需 `system_runtime_manage`。

响应：200 `{"success":true,"deleted":N}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/runtime/queues/default/archived -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/tenants/apply-default-storage-quota

用途：把当前默认存储配额批量写到全部空间（平台 key 需 `system_tenants_manage`）。无请求体。

响应：200 `{"affected":N,"quota_bytes":N,"quota_gb":N}`

```bash
curl -X POST $BASE/api/v1/system/admin/tenants/apply-default-storage-quota -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/audit-log

用途：平台级审计日志（tenant_id=0 行；平台 key 需 `system_audit_read`）。查询参数同空间审计（`after_id/limit/action/outcome/actor`）。Handler: `internal/handler/audit_log.go`

响应：200 `{"success":true,"data":[AuditLog],"next_cursor":N}`

```bash
curl $BASE/api/v1/system/admin/audit-log -H "Authorization: Bearer $TOKEN"
```
