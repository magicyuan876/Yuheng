# API 参考：组织与共享

路由注册：`internal/router/routes_organization.go` 的 `RegisterOrganizationRoutes`。Handler：`internal/handler/organization.go`。

组织（Organization）以“空间（tenant）”为成员单位。成员空间在组织内的角色（`OrgMemberRole`）取 `viewer` / `editor` / `admin`；创建组织的空间是属主空间（owner tenant），不能被移除，也不能改角色。组织组路由的 API key 策略为 `manage_spaces` 或 full-access；KB 分享管理仅 full-access key 可用。组织成员之间共享的资源只有知识库。

## 组织管理（/api/v1/organizations）

### POST /api/v1/organizations

用途：创建组织。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 组织名称，1–255 字符 |
| `description` | string | 否 | 描述，最多 1000 字符 |
| `avatar` | string | 否 | 头像 URL，最多 512 字符 |
| `member_limit` | int | 否 | 成员空间数上限，`0` 为不限，默认 50 |
| `invite_code_validity_days` | int | 否 | 邀请码有效期（天）：`0` 永不过期，或 1 / 7 / 30，默认 7 |

“可被搜索发现”（`searchable`）和“加入需审批”（`require_approval`）只能在创建后通过更新接口设置。

响应：201 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl -X POST $BASE/api/v1/organizations -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"研发组织"}'
```

### GET /api/v1/organizations

用途：列出我所在的组织。权限：Viewer+。

响应：200 `{"success":true,"data":{"organizations":[...],"total":N,"resource_counts":{"knowledge_bases":{"by_organization":{"<org_id>":N}}}}}`

```bash
curl $BASE/api/v1/organizations -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/preview/:code

用途：按邀请码预览组织（不加入），调用者不必是成员，用于“加入前预览”页。权限：Viewer+。路径参数：`code` 邀请码。

响应：200 `{"success":true,"data":{id,name,description,avatar,member_count,share_count,is_already_member,require_approval,created_at}}`

```bash
curl $BASE/api/v1/organizations/preview/ABC123 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/join

用途：凭邀请码加入组织，以 `viewer` 身份加入。只适用于不需要审批的组织，需要审批的组织走 `join-request`。权限：Admin+。请求体：`{"invite_code":"..."}`（必填，8–32 字符）。

响应：200 `{"success":true,"data":{OrganizationResponse}}`。邀请码无效、已过期或组织需要审批时统一返回 404 `Invalid invite code`；组织成员已满返回 1010。

```bash
curl -X POST $BASE/api/v1/organizations/join -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"invite_code":"ABC123"}'
```

### POST /api/v1/organizations/join-request

用途：提交加入申请（需审批的组织）。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `invite_code` | string | 是 | 邀请码，8–32 字符 |
| `message` | string | 否 | 申请附言，最多 500 字符 |
| `role` | string | 否 | 期望角色：`viewer` / `editor` / `admin` |

响应：200 `{"success":true,"data":{JoinRequest}}`。组织不需要审批、本空间已是成员、角色非法、成员已满或已有待审申请时返回 1010；邀请码无效返回 404。

```bash
curl -X POST $BASE/api/v1/organizations/join-request -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"invite_code":"ABC123","message":"申请加入"}'
```

### GET /api/v1/organizations/search

用途：搜索可发现（`searchable:true`）的组织，按名称、描述或组织 ID 模糊匹配；只返回元数据，不含邀请码。权限：Viewer+。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `q` | string | 否 | 关键字 |
| `limit` | int | 否 | 默认 20，上限 100 |

响应：200 `{"success":true,"data":[SearchableOrganization],"total":N}`

```bash
curl "$BASE/api/v1/organizations/search?q=研发" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/join-by-id

用途：按组织 ID 加入可发现组织（无需邀请码）。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `organization_id` | string | 是 | 目标组织 ID |
| `message` | string | 否 | 附言，最多 500 字符 |
| `role` | string | 否 | 期望角色：`viewer` / `editor` / `admin`，默认 `viewer`；仅在需要审批时写进申请 |

目标组织必须是可发现的，否则拒绝。组织需要审批时会创建加入申请；不需要审批时直接以 `viewer` 加入。本空间已是成员时幂等返回。

响应：200 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl -X POST $BASE/api/v1/organizations/join-by-id -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"organization_id":"org-1"}'
```

### GET /api/v1/organizations/:id

用途：组织详情。权限：Viewer+。

响应：200 `{"success":true,"data":{OrganizationResponse}}`。`my_role` 是本空间在组织内的角色；`invite_code`、`invite_code_expires_at` 与待审申请数只在本空间是组织 admin 或属主空间时返回。

```bash
curl $BASE/api/v1/organizations/org-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/organizations/:id

用途：更新组织（服务层校验调用者空间为组织 owner）。权限：Admin+。请求体字段均可选：`name`、`description`、`avatar`、`member_limit`、`invite_code_validity_days`，以及 `require_approval`（加入是否需审批）、`searchable`（是否可被搜索发现）。

响应：200 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl -X PUT $BASE/api/v1/organizations/org-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"description":"更新描述"}'
```

### DELETE /api/v1/organizations/:id

用途：删除组织，同时删除共享到该组织的全部 KB 分享记录。权限：Admin+，且服务层要求调用者空间是属主空间。

响应：200 `{"success":true,"message":"Organization deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/organizations/org-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/:id/leave

用途：本空间退出组织。属主空间不能退出。权限：Admin+。无请求体。

响应：200 `{"success":true,"message":"Left organization successfully"}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/leave -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/:id/request-upgrade

用途：申请提升本空间在组织内的角色，等待组织 admin 审批；申请出现在 `join-requests` 里，`request_type` 为 `upgrade`。不能申请相同或更低的角色。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `requested_role` | string | 是 | 期望的组织角色（`viewer/editor/admin`） |
| `message` | string | 否 | 附言，最多 500 字符 |

响应：200 `{"success":true,"data":{JoinRequest}}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/request-upgrade -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"requested_role":"editor"}'
```

### POST /api/v1/organizations/:id/invite-code

用途：重新生成组织邀请码，旧邀请码随之失效。权限：Admin+（服务层要求组织 admin）。无请求体。

响应：200 `{"success":true,"data":{"invite_code":"..."}}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/invite-code -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/search-tenants

用途：搜索可邀请的空间（返回按空间分组的候选）。权限：Admin+。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `q` | string | 是 | 空间名关键字 |
| `limit` | int | 否 | 默认 10，上限 50 |

响应：200 `{"success":true,"data":[{"tenant_id","tenant_name"}]}`

```bash
curl "$BASE/api/v1/organizations/org-1/search-tenants?q=demo" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/:id/invite

用途：直接邀请空间加入组织。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `tenant_id` | uint64 | 是 | 目标空间 ID |
| `representative_user_id` | string | 否 | 该空间的代表用户（仅用于展示与审计；不属于该空间的用户会被忽略） |
| `role` | string | 是 | 组织内角色 |

响应：200 `{"success":true,"message":"Member added successfully"}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/invite -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"tenant_id":2,"role":"viewer"}'
```

### GET /api/v1/organizations/:id/members

用途：组织成员（空间）列表。权限：Viewer+。

响应：200 `{"success":true,"data":{"members":[{id,representative_user_id,role,tenant_id,tenant_name,username,email,avatar,joined_at}],"total":N}}`

```bash
curl $BASE/api/v1/organizations/org-1/members -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/organizations/:id/members/:tenant_id

用途：修改成员空间的组织角色。权限：Admin+。路径参数 `tenant_id` 为成员空间 ID。请求体：`{"role":"editor"}`（必填，`viewer/editor/admin`）。

响应：200 `{"success":true,"message":"Member role updated successfully"}`

```bash
curl -X PUT $BASE/api/v1/organizations/org-1/members/2 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"editor"}'
```

### DELETE /api/v1/organizations/:id/members/:tenant_id

用途：移除成员空间（含自移除）。属主空间不能被移除。权限：Admin+。

响应：200 `{"success":true,"message":"Member removed successfully"}`

```bash
curl -X DELETE $BASE/api/v1/organizations/org-1/members/2 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/join-requests

用途：待审批的申请队列，只返回 `status=pending` 的记录；`request_type` 区分新加入（`join`）与角色升级（`upgrade`）。权限：Admin+。

响应：200 `{"success":true,"data":{"requests":[{id,user_id,username,email,message,request_type,prev_role,requested_role,status,created_at,reviewed_at}],"total":N}}`

```bash
curl $BASE/api/v1/organizations/org-1/join-requests -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/organizations/:id/join-requests/:request_id/review

用途：审批加入/升级申请。权限：Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `approved` | bool | 是 | 通过/拒绝 |
| `message` | string | 否 | 审批意见，最多 500 字符 |
| `role` | string | 否 | 通过时授予的角色（`viewer` / `editor` / `admin`）；缺省用申请里的期望角色，都没有时为 `viewer` |

响应：200 `{"success":true,"message":"Review completed"}`

```bash
curl -X PUT $BASE/api/v1/organizations/org-1/join-requests/req-1/review \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"approved":true}'
```

### GET /api/v1/organizations/:id/shares

用途：查看共享到该组织的 KB 列表。权限：Viewer+。每项的 `my_permission` 是调用者的有效权限，等于分享权限与本空间组织角色中较低的一个。

响应：200 `{"success":true,"data":{"shares":[KnowledgeBaseShareResponse],"total":N}}`

```bash
curl $BASE/api/v1/organizations/org-1/shares -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/shared-knowledge-bases

用途：组织空间视图：组织内全部共享 KB，包括别人共享进来的和本空间共享出去的（`is_mine:true`），供切换到该组织后的知识库列表页使用。权限：Viewer+。

响应：200 `{"success":true,"data":[{knowledge_base,share_id,organization_id,org_name,permission,source_tenant_id,shared_at,is_mine}],"total":N}`

```bash
curl $BASE/api/v1/organizations/org-1/shared-knowledge-bases -H "Authorization: Bearer $TOKEN"
```

## KB 分享（/api/v1/knowledge-bases/:id/shares）

API key：仅 full-access。Handler: `internal/handler/organization.go`

### POST /api/v1/knowledge-bases/:id/shares

用途：把 KB 分享到组织。权限：KB 创建者 OR Admin+。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `organization_id` | string | 是 | 目标组织 |
| `permission` | string | 是 | 共享权限（组织角色语义，如 `viewer/editor`） |

只有 KB 的属主空间能分享，且该空间在目标组织里至少是 `editor`；同一 KB 重复分享到同一组织会被拒绝。

响应：201 `{"success":true,"data":{KBShare}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/shares -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"organization_id":"org-1","permission":"viewer"}'
```

### GET /api/v1/knowledge-bases/:id/shares

用途：查看该 KB 被分享到的全部组织。权限：Viewer+。

响应：200 `{"success":true,"data":{"shares":[KnowledgeBaseShareResponse],"total":N}}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/shares -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id/shares/:share_id

用途：修改分享权限。权限：KB 创建者 OR Admin+。请求体：`{"permission":"editor"}`（必填）。

响应：200 `{"success":true,"message":"Share permission updated successfully"}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/shares/s-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"permission":"editor"}'
```

### DELETE /api/v1/knowledge-bases/:id/shares/:share_id

用途：取消分享。权限：KB 创建者 OR Admin+。

响应：200 `{"success":true,"message":"Share removed successfully"}`

```bash
curl -X DELETE $BASE/api/v1/knowledge-bases/kb-1/shares/s-1 -H "Authorization: Bearer $TOKEN"
```

## 共享资源聚合视图

### GET /api/v1/shared-knowledge-bases

用途：列出通过组织共享给我的 KB（去除属主侧向量库元数据）。权限：Viewer+；API key 需 `manage_spaces` 或 full-access。

响应：200 `{"success":true,"data":[SharedKnowledgeBaseInfo],"total":N}`（`knowledge_base,share_id,organization_id,org_name,permission,source_tenant_id,shared_at`）

```bash
curl $BASE/api/v1/shared-knowledge-bases -H "Authorization: Bearer $TOKEN"
```
