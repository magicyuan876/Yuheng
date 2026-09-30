# API 参考：租户（空间）与成员

路由注册：`internal/router/routes_auth_tenant.go` 的 `RegisterTenantRoutes`（KB 活动流在 `internal/router/routes_knowledge.go` 的 `RegisterKnowledgeBaseActivityRoutes`）。Handler：`internal/handler/tenant.go`、`internal/handler/tenant_member.go`、`internal/handler/tenant_invitation.go`、`internal/handler/tenant_invite_link.go`、`internal/handler/audit_log.go`。

所有 `/tenants/:id/*` 路由在组级挂载 `PathTenantMatch()`（`internal/middleware/access.go`）：URL 中的 `:id` 必须等于当前活跃空间（跨空间超管例外），防止越权操作他人空间。

## 空间生命周期

### POST /api/v1/tenants

用途：创建空间（自助开新工作区；调用者自动成为 Owner）。权限：任何已登录用户（可无空间）；API key 仅平台 key 且具 `system_tenants_manage`。Handler: `internal/handler/tenant.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是（`binding:"required,min=1,max=128"`） | 空间名称 |
| `description` | string | 否（`binding:"max=512"`） | 描述 |

跨空间超管可提交完整 `types.Tenant`（含 `storage_quota`、`status` 等）。

响应：201 `{"success":true,"data":{Tenant}}`。默认不发放 API key，需要时创建后再调 `POST /tenants/:id/api-keys`。系统设置 `tenant.auto_create_api_key`（环境变量 `YUHENG_TENANT_AUTO_CREATE_API_KEY`，默认 false）打开时恢复旧行为：创建空间顺带生成一个 `full_access` key，明文只在这次响应的 `data.api_key` 里返回。自助创建被禁用返回 403（code 2005），超配额返回 429。

```bash
curl -X POST $BASE/api/v1/tenants -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"我的空间"}'
```

### GET /api/v1/tenants

用途：列出我可访问的空间。权限：已登录；API key 需 `manage_tenant_settings` 或 full-access。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"data":{"items":[TenantResponse]}}`

```bash
curl $BASE/api/v1/tenants -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/all

用途：列出全部空间（跨空间超管）。权限：`CrossTenant()`（`CanAccessAllTenants` 且集群开启 `EnableCrossTenantAccess`）；平台 key 需 `system_tenants_read|manage`。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"data":{"items":[TenantResponse]}}`

```bash
curl $BASE/api/v1/tenants/all -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/search

用途：按关键字搜索空间（跨空间超管）。权限：同上。Handler: `internal/handler/tenant.go`

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `keyword` | string | 否 | 关键字 |
| `tenant_id` | string | 否 | 精确空间 ID |
| `page` / `page_size` | int | 否 | 分页（默认 1/20，上限 100） |

响应：200 `{"success":true,"data":{"items":[...],"total","page","page_size"}}`

```bash
curl "$BASE/api/v1/tenants/search?keyword=demo&page=1" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/:id

用途：空间详情。权限：Viewer+；平台 key 需 `system_tenants_read|manage`。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"data":{TenantResponse}}`

```bash
curl $BASE/api/v1/tenants/1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/tenants/:id

用途：更新空间配置。权限：Owner；平台 key 需 `system_tenants_manage`。Handler: `internal/handler/tenant.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | *string | 否（`binding:"omitempty,min=1,max=128"`） | 新名称 |
| `description` | *string | 否（`binding:"omitempty,max=512"`） | 新描述 |

响应：200 `{"success":true,"data":{TenantResponse}}`

```bash
curl -X PUT $BASE/api/v1/tenants/1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"新名字"}'
```

### DELETE /api/v1/tenants/:id

用途：删除空间。权限：Owner；平台 key 需 `system_tenants_manage`。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"message":"Workspace deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1 -H "Authorization: Bearer $TOKEN"
```

## 空间 KV 配置

`:key` 为配置键而非空间 ID（空间取自认证上下文，URL 里不接受 tenant_id），可选值如下，其他键返回 400 `unsupported key`：

| key | 内容 | 读写要求 |
| --- | --- | --- |
| `web-search-config` | 网页搜索配置 | 读写都要 Admin+（API key 需 full-access 或 `manage_tenant_settings`），否则 403；开启集中管控后只有系统管理员能写 |
| `parser-engine-config` | 空间级解析引擎覆盖 | 同上 |
| `storage-engine-config` | 存储引擎配置（local / s3） | 同上 |
| `chat-history-config` | 聊天历史索引配置 | 读 Viewer+，写 Admin+ |
| `retrieval-config` | 全局检索配置 | 读 Viewer+，写 Admin+ |
| `prompt-templates` | 系统提示词模板，按请求语言本地化 | 只读，不支持 PUT |

写入时的校验：

- `web-search-config`：`max_results` 取 1-50。
- `retrieval-config`：`vector_threshold`、`keyword_threshold` 取 0-1，`rerank_threshold` 取 -10 到 10，`embedding_top_k`、`rerank_top_k` 取 0-200。
- `storage-engine-config`：`default_provider` 必须在 `STORAGE_ALLOW_LIST` 允许的列表内（缺省时取列表里第一个）。
- `chat-history-config`：启用、设置了 `embedding_model_id` 且还没有关联知识库时，会自动建一个隐藏知识库并把它的 ID 写回配置；换了 embedding 模型则不沿用旧知识库。

### GET /api/v1/tenants/kv/:key

用途：读取空间级 KV 配置。权限：Viewer+；API key 需 `manage_tenant_settings` 或 full-access。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"data":{...对应配置对象...}}`

```bash
curl $BASE/api/v1/tenants/kv/retrieval-config -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/tenants/kv/:key

用途：更新空间级 KV 配置。权限：Admin+；API key 需 `manage_tenant_settings` 或 full-access。请求体：与 `:key` 对应的配置 JSON 对象。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"message":"Configuration updated"}`

```bash
curl -X PUT $BASE/api/v1/tenants/kv/web-search-config -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"enabled":true}'
```

## API Key 与 API 主体

### GET /api/v1/tenants/:id/api-keys

用途：列出空间 API key（掩码显示）。权限：Owner，仅 JWT（API key 默认拒绝）。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"data":[{id,scope_type,name,api_key(掩码),full_access,knowledge_base_ids,capabilities,last_used_at,expires_at,created_at}]}`

```bash
curl $BASE/api/v1/tenants/1/api-keys -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/api-keys

用途：创建空间 API key（明文仅返回一次）。权限：Owner，仅 JWT。Handler: `internal/handler/tenant.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | key 名称，不能为空白 |
| `full_access` | bool | 否 | 空间全权 key（默认 false） |
| `knowledge_base_ids` | []string | 否 | KB 白名单（scoped key）；每个 ID 必须存在且属于本空间，空列表表示不限 KB |
| `capabilities` | []string | scoped key 必填 | capability 列表（见[总览](./01-api-overview.md)）；`full_access=false` 时至少一个，出现未知值返回 1010 |
| `expires_at_unix` | *int64 | 否 | 过期时间（Unix 秒） |

响应：201 `{"success":true,"data":{...,"api_key":"sk-AbCd...wXyZ","token":"<明文>"}}`。`token` 是 Key 本身，只在这次响应里出现；服务端只保存它的哈希和 `api_key` 这个掩码提示，之后无从找回。

```bash
curl -X POST $BASE/api/v1/tenants/1/api-keys -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"ingest-bot","capabilities":["ingest","retrieve"],"knowledge_base_ids":["kb-1"]}'
```

### PUT /api/v1/tenants/:id/api-keys/:key_id

用途：修改已创建 key 的名称、全权开关、KB 白名单、capability 与过期时间，key 本身不变。权限：Owner，仅 JWT。请求体字段与创建接口相同，校验规则也相同（整体替换，不是局部合并）。

响应：200 `{"success":true,"data":{tenantAPIKeyResponse}}`。key 不存在（或不属于该空间）返回 404；请求不合法返回 400，例如给空间 Key 申请 `system_*` 能力（这类能力只属于平台 Key）。

```bash
curl -X PUT $BASE/api/v1/tenants/1/api-keys/5 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"ingest-bot","capabilities":["ingest","retrieve","manage_kbs"],"knowledge_base_ids":["kb-1"]}'
```

### DELETE /api/v1/tenants/:id/api-keys/:key_id

用途：删除 API key。权限：Owner，仅 JWT。路径参数：`key_id`。

响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1/api-keys/5 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/:id/api-principal-config

用途：读取 API 外部用户主体配置。权限：Owner，仅 JWT。Handler: `internal/handler/tenant.go`

响应：200 `{"success":true,"data":{"mode":"tenant|direct_header|signed_token","direct_header_name","signed_token_header_name","require_direct_header","has_hmac_secret"}}`

```bash
curl $BASE/api/v1/tenants/1/api-principal-config -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/tenants/:id/api-principal-config

用途：更新 API 外部用户主体配置。权限：Owner，仅 JWT。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `mode` | string | 是 | `tenant` / `direct_header` / `signed_token`，其他值返回 1010 |
| `require_direct_header` | bool | 否 | `direct_header` 模式下缺 Header 时是否返回 401（否则回落为空间级主体） |
| `hmac_secret` | *string | 否 | signed_token 模式密钥（传 `***` 保留原值） |

切到 `signed_token` 时必须已有或本次提供 `hmac_secret`，否则返回 1010。各模式的安全假设见[总览](./01-api-overview.md)的「外部用户主体」：`direct_header` 的用户 ID 可被任何持有 key 的调用方伪造，面向终端用户的集成应使用 `signed_token`。

Header 名固定为 `X-External-User-ID` 与 `X-External-User-Token`：请求里的 `direct_header_name` / `signed_token_header_name` 会被忽略，服务端总是写回默认值。

响应：200，同 GET。

```bash
curl -X PUT $BASE/api/v1/tenants/1/api-principal-config -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"mode":"signed_token","hmac_secret":"topsecret"}'
```

### POST /api/v1/tenants/:id/api-principal-test-token

用途：签发用于测试的外部用户 JWT。权限：Owner，仅 JWT。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `external_user_id` | string | 是 | 外部用户 ID（≤128 字符） |
| `expires_in_seconds` | int | 否 | 1-3600，默认 900 |

响应：200 `{"success":true,"data":{"token","header_name","expires_in_seconds","expires_at_unix","external_user_id"}}`

```bash
curl -X POST $BASE/api/v1/tenants/1/api-principal-test-token -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"external_user_id":"u-123"}'
```

## 成员管理（/tenants/:id/members）

Handler: `internal/handler/tenant_member.go`。API key 需 `manage_members` 或 full-access。

### GET /api/v1/tenants/:id/members

用途：成员列表。权限：Viewer+。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `q` | string | 否 | 邮箱/用户名过滤 |
| `page` / `page_size` | int | 否 | 分页 |

响应：200 `{"success":true,"data":{"members":[{user_id,email,username,avatar,role,status,invited_by,joined_at}],"total","page","page_size"}}`

```bash
curl $BASE/api/v1/tenants/1/members -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/members

用途：直接添加成员。权限：Owner。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `email` | string | 是（`binding:"required,email"`） | 成员邮箱（须已注册） |
| `role` | string | 是（`binding:"required"`） | `owner/admin/contributor/viewer` |

响应：201 `{"success":true,"data":{成员对象}}`

```bash
curl -X POST $BASE/api/v1/tenants/1/members -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"b@ex.com","role":"contributor"}'
```

### PUT /api/v1/tenants/:id/members/:user_id

用途：修改成员角色。权限：Owner。请求体：`{"role":"admin"}`（`binding:"required"`）。

响应：200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/tenants/1/members/u-123 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"admin"}'
```

### DELETE /api/v1/tenants/:id/members/:user_id

用途：移除成员。权限：Owner。

响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1/members/u-123 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/leave

用途：退出空间（任何成员可自行退出；服务层拒绝导致空间无 Owner 的退出）。权限：Viewer+，仅 JWT。

响应：200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/tenants/1/leave -H "Authorization: Bearer $TOKEN"
```

## 空间邀请（/tenants/:id/invitations 与 invite-links）

Handler: `internal/handler/tenant_invitation.go`、`internal/handler/tenant_invite_link.go`。API key 需 `manage_members` 或 full-access。

### GET /api/v1/tenants/:id/invitations

用途：空间邀请列表。权限：Viewer+。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `include_terminal` | bool | 否 | 包含已完结邀请 |
| `page` / `page_size` | int | 否 | 分页 |

响应：200 `{"success":true,"data":{"invitations":[{id,tenant_id,invitee_email,inviter_email,role,status,message,expires_at,is_share_link,accepted_count,...}],"total","page","page_size"}}`

```bash
curl $BASE/api/v1/tenants/1/invitations -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/invitations

用途：按邮箱邀请**已注册**用户。默认生成一条 pending 邀请，被邀请人在 `/me/invitations` 确认后才成为成员。权限：Owner。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `email` | string | 是（`binding:"required,email"`） | 被邀请邮箱 |
| `role` | string | 是（`binding:"required"`） | 授予角色 |
| `message` | string | 否 | 附言 |

响应：201 `{"success":true,"data":{TenantInvitationResponse}}`；已有待处理邀请或对方已是成员返回 409。

系统设置 `tenant.auto_accept_invitation`（环境变量 `YUHENG_TENANT_AUTO_ACCEPT_INVITATION`，默认 false）打开时跳过确认：直接写入成员关系并清理对方已有的 pending 邀请，响应的 `data` 是成员对象（含 `user_id`、`status:"active"`），不再是邀请对象。受邀人没有默认空间时，这个空间成为其默认空间。客户端可以从 `GET /auth/me` 的 `capabilities.auto_accept_invitation` 得知当前行为。

```bash
curl -X POST $BASE/api/v1/tenants/1/invitations -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"c@ex.com","role":"viewer"}'
```

### DELETE /api/v1/tenants/:id/invitations/:inv_id

用途：撤销邀请。权限：Owner。

响应：200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1/invitations/12 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/invite-links

用途：创建分享链接（多次可用的注册邀请链接）。权限：Owner。Handler: `internal/handler/tenant_invite_link.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `role` | string | 是（`binding:"required"`） | 链接授予的角色 |
| `message` | string | 否 | 附言 |

响应：201 `{"success":true,"data":{id,token,invite_url,role,status,expires_at,is_share_link:true,accepted_count}}`

```bash
curl -X POST $BASE/api/v1/tenants/1/invite-links -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"viewer"}'
```

## 审计日志

Handler: `internal/handler/audit_log.go`。游标分页。

### GET /api/v1/tenants/:id/audit-log

用途：空间审计日志（含被拒绝操作记录）。权限：Admin+，仅 JWT。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `after_id` | int | 否 | 游标（上次响应 `next_cursor`） |
| `limit` | int | 否 | 1-100，默认 50 |
| `action` | string | 否 | 按动作过滤（如 `rbac.member_added`） |
| `outcome` | string | 否 | `success` / `denied` |
| `actor` | string | 否 | 按操作者 user_id 过滤 |

响应：200 `{"success":true,"data":[AuditLog],"next_cursor":N}`

```bash
curl "$BASE/api/v1/tenants/1/audit-log?limit=50" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/activity

用途：单个 KB 的活动流（只读审计）。权限：KB 创建者 OR Admin+，且对 KB 有 read 权限；仅 JWT。查询参数同上（`after_id/limit/action/outcome/actor`）。注册于 `RegisterKnowledgeBaseActivityRoutes`。

响应：200 `{"success":true,"data":[AuditLog],"next_cursor":N}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/activity -H "Authorization: Bearer $TOKEN"
```
