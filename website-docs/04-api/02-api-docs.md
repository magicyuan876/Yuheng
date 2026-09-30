# API 参考：在线文档

路由注册：`internal/router/routes_docs.go` 的 `RegisterDocsRoutes`（`/api/v1/docs/**`、`/api/v1/groups/**`）、`RegisterDocsPublicRoutes`（匿名的 `/api/v1/docs/public/**`、`/api/v1/docs/public-spaces/**`）与 `RegisterDocsInternalRoutes`（协同服务回调 `/internal/collab/**`）。Handler：`internal/docs/handler/*.go`，业务规则在 `internal/docs/service/`。

空间、页面限制、锁定、独占编辑、知识库同步等概念见功能页[在线文档](../03-features/07-docs.md)，本页只写接口。模块默认关闭：只有 `YUHENG_DOCS_ENABLED=true` 时才注册这些路由，关闭时所有路径都是 404；匿名路由另需 `YUHENG_DOCS_PUBLIC_SHARING=true`，协同回调另需 `YUHENG_COLLAB_SHARED_SECRET`。

## 通用约定

### 三道权限

每个路由依次经过三层判断：

1. **API key capability**（只对 API key 生效）：读路由要 `docs_read`，写路由要 `docs_write`，管理类路由要 `docs_admin`；full-access key 全部可用。下文用「key `docs_read`」等标注。`/groups/**` 全部路由（包括读）都要 `docs_admin`。
2. **工作区角色下限**：`Viewer+` / `Contributor+` / `Admin+`，与其它模块相同。评论、收藏、关注、导出这些「读者就能做」的写操作只要 Viewer。
3. **文档 ACL 守卫**：调用者在空间或页面上的有效角色（`reader` / `writer` / `admin`，算法见功能页 2.2）。下文写作「空间 reader」「页面 writer」等；「成员」表示守卫只要求是工作区活跃成员，其余判断在服务层逐条完成。

守卫的回答：

| 情况 | 响应 |
| --- | --- |
| 看不到的空间或页面（包括不存在） | 404 `{"error":"not found"}`，不用 403，避免泄露存在性 |
| 看得到但角色不够 | 403 `{"error":"Forbidden: insufficient permission on this document"}` |
| 页面在回收站且调用者原本能读 | 410 `{"success":false,"error":"page is in the trash","data":{page_id,short_id,space_id,title,deleted_at,deleted_by,restorable}}`，`restorable` 表示调用者是否有 writer |
| 请求没有用户身份 | 401 `Unauthorized: docs routes require a user session` |
| 身份不是本工作区活跃成员 | 403 `Forbidden: not an active member of this workspace` |
| 权限解析失败（数据库或缓存故障） | 503 `{"error":"permission check unavailable"}` |

API key 在本模块里以**自己的身份**解析权限，不借用任何真人账号：身份 ID 为 `api_tenant_key:<工作区ID>:<keyID>`（解析出外部用户时为 `api_external_user:<工作区ID>:<外部用户ID>`，平台级 key 为 `api_platform:<keyID>`），页面的作者、审计记录都记在这个 ID 下。它能看到的是工作区给「所有人」的那部分，再按能力封顶：

| key 的能力 | 相当于 | 能看到、能做 |
| --- | --- | --- |
| `docs_read` | 工作区 Viewer | 开放与公开空间、授权给默认组「所有人」的空间和页面，最多 reader |
| `docs_write` | 工作区 Contributor | 同上，角色按空间的默认角色或「所有人」的授权，最高 writer |
| `docs_admin` 或完全访问 | 工作区 Admin | 所有空间的 admin，包括私有空间与受限页面 |

- 私有空间和只授权给具体人员的受限页面，非管理员 key 看不到：没有办法把权限授给一个 key。
- key 限定了知识库时，只能看到绑定到这些知识库之一的空间；即使是 `docs_admin`，也只是这些空间的 admin，不能管理用户组、工作区模板和已删除的空间。
- key 不会被自动加入页面关注，也不会收到通知。

### 响应与错误

- 成功：`{"success":true,"data":...}`，状态码 200；创建空间、页面、复制页面、上传附件、创建用户组为 201；导入与空间导出这类后台任务为 202；删除空间、成员、附件、用户组等无返回内容的操作为 204（无响应体）。
- 服务层错误走统一错误中间件：`{"success":false,"error":{"code":...,"message":"...","details":null}}`。常见：参数不合法 400（code 1010）、权限不足 403（1002）、不存在 404（1003）、冲突 409（1005）、配额超限 409。
- 本模块里 `repository.ErrDuplicate` 映射为 400 `already exists`，版本冲突为 409 `version conflict; reload and retry`。
- 未知错误只记日志，返回 500 `internal error`，不带原始错误文本。

### 幂等与分页

- 写操作支持 `Idempotency-Key` 请求头（≤128 字符）：同一用户、同一路由、同一 key 在 24 小时内重放第一次的成功响应（带响应头 `Idempotent-Replayed: true`）；前一个同 key 请求还在执行时返回 409。只存 2xx 且响应体不超过 1 MiB 的结果。以下写操作**没有**这个中间件：附件上传、导入、租约获取与释放、Yjs 保存、维护清理，以及匿名路由。
- 列表分页多为游标式：请求带 `cursor`（或 `after`）与 `limit`，响应带 `next_cursor`，为空表示没有下一页。用户组成员列表例外，用 `page` / `page_size`。

```bash
BASE=http://localhost:8080
TOKEN=<JWT>
```

## 事件流

### GET /api/v1/docs/events

用途：SSE 事件流，作为「该刷新了」的提示，客户端应重新拉取数据，不要把事件内容当作真相。权限：Viewer+，成员；key `docs_read`。

每个事件推送前都按订阅者**当前**的权限过滤，与发起一次请求的判定相同：

- 通知（`docs.notification.created`）只推给接收人；
- 页面事件推给能读该页面的人；页面已被彻底删除，或已移到别的空间（从原空间看的那一条），则推给能读事件所在空间的人，这时只带 ID；
- 空间事件推给能读该空间的人；不带空间和页面的事件（用户组、工作区模板、缓存失效）只带 ID，推给所有成员；
- 连接期间失去某个页面或空间的权限时，只会再收到让它消失的那一个事件，之后的事件不再推送；
- 订阅者被移出工作区时，连接直接结束。

| 查询参数 | 说明 |
| --- | --- |
| `space` | 只收该空间的事件（不带 `space_id` 的事件照常收到） |
| `page` | 只收该页面的事件 |

连接建立后先发 `event: ready`（`data: {"tenant_id":N}`），此后每 25 秒一行 `: ping` 心跳。每个事件的 `event:` 是类型名，`data:` 是 `{type,tenant_id,space_id,page_id,actor_id,at,payload}`。类型有 `docs.space.created|updated|deleted|members_changed`、`docs.page.created|content_updated|meta_updated|moved|deleted|restored|purged|access_changed|content_replaced|tree_rebalanced|lease_changed`、`docs.revision.created`、`docs.attachment.added`、`docs.comment.changed`、`docs.notification.created`、`docs.share.changed`、`docs.template.changed`、`docs.label.changed`、`docs.group.changed`、`docs.acl.invalidated`。慢客户端积压超过 64 条时丢弃事件。事件总线未配置时返回 503。

```bash
curl -N "$BASE/api/v1/docs/events?space=<sid>" -H "Authorization: Bearer $TOKEN"
```

## 空间（/api/v1/docs/spaces）

返回的空间对象（`SpaceView`）为空间行字段 `id`、`slug`、`name`、`description`、`icon`、`visibility`、`default_role`、`knowledge_base_id`、`storage_backend_id`、`settings`、`quota_bytes`、`creator_id`、`created_at`、`updated_at`，外加调用者的 `role` 以及 `member_count`、`page_count`。

### GET /api/v1/docs/spaces

用途：列出调用者可读的空间及其在每个空间中的角色（成员空间、开放与公开空间；工作区 Owner/Admin 看到全部）。权限：Viewer+，成员；key `docs_read`。

响应：200 `{"success":true,"data":[SpaceView]}`

```bash
curl $BASE/api/v1/docs/spaces -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/docs/spaces

用途：创建空间，创建者成为空间管理员。权限：Contributor+，成员；key `docs_write`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 名称，单行，最多 100 字 |
| `slug` | string | 否 | 路径标识，2–64 位小写字母、数字、连字符；省略时由名称生成并保证唯一 |
| `description` | string | 否 | 最多 4000 字 |
| `icon` | string | 否 | 最多 64 字节 |
| `visibility` | string | 否 | `private`（默认）/ `open` / `public`；`public` 需部署开启公开分享，否则 403 |
| `default_role` | string | 否 | 开放、公开空间的默认角色 `reader`（默认）/ `writer`；私有空间固定为 `none` |
| `knowledge_base_id` | string | 否 | 绑定的知识库，必须属于本工作区 |
| `storage_backend_id` | string | 否 | 附件使用的存储后端，必须属于本工作区 |
| `settings` | object | 否 | 自由 JSON 对象，最多 16 KiB |

响应：201 `{"success":true,"data":{SpaceView}}`。显式给的 slug 已被占用返回 409；知识库或存储后端不在本工作区返回 400。

```bash
curl -X POST $BASE/api/v1/docs/spaces -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"产品手册","slug":"handbook","visibility":"open"}'
```

### GET /api/v1/docs/spaces/:sid · GET /api/v1/docs/spaces/by-slug/:slug

用途：按 ID 或 slug 读取空间。权限：Viewer+，空间 reader；key `docs_read`。

响应：200 `{"success":true,"data":{SpaceView}}`

```bash
curl $BASE/api/v1/docs/spaces/by-slug/handbook -H "Authorization: Bearer $TOKEN"
```

### PATCH /api/v1/docs/spaces/:sid

用途：修改空间。权限：Contributor+，空间 admin；key `docs_write`。

请求体字段均可选，缺省表示不变：`name`、`slug`、`description`、`icon`（空字符串清除）、`visibility`、`default_role`、`settings`，取值规则同创建。

响应：200 `{"success":true,"data":{SpaceView}}`；slug 冲突 409。

```bash
curl -X PATCH $BASE/api/v1/docs/spaces/<sid> -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"description":"对外产品文档"}'
```

### DELETE /api/v1/docs/spaces/:sid

用途：软删除空间，其页面随之不可访问；不会被自动物理清理。权限：Contributor+，空间 admin；key `docs_write`。

响应：204

### POST /api/v1/docs/spaces/:sid/restore

用途：恢复已删除的空间（前端没有入口）。权限：Contributor+，成员，服务层要求**工作区** Owner/Admin，否则 403；key `docs_write`。

响应：200 `{"success":true,"data":{SpaceView}}`；slug 已被别的空间占用时 409 `another space now uses this slug; rename it first`。

## 空间成员与知识库绑定

### GET /api/v1/docs/spaces/:sid/members

用途：列出直接成员（用户与用户组），管理员在前，同一角色内用户组在前。权限：Viewer+，空间 reader；key `docs_read`。

响应：200 `{"success":true,"data":[{principal_type,principal_id,role,name,email,avatar,is_default_group,group_member_count,added_by,created_at}]}`

### PUT /api/v1/docs/spaces/:sid/members

用途：添加成员或修改角色，幂等（已是成员的更新为新角色）。权限：Contributor+，空间 admin；key `docs_write`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `members` | array | 是 | 最多 200 项 |
| `members[].principal_type` | string | 是 | `user` / `group` |
| `members[].principal_id` | string | 是 | 用户 ID 或用户组 ID |
| `members[].role` | string | 是 | `reader` / `writer` / `admin` |

响应：200，返回更新后的成员列表（同上）。用户不是本工作区活跃成员、用户组不存在、改动后空间没有管理员，都返回 400。

```bash
curl -X PUT $BASE/api/v1/docs/spaces/<sid>/members -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"members":[{"principal_type":"user","principal_id":"u-1","role":"writer"}]}'
```

### DELETE /api/v1/docs/spaces/:sid/members/:ptype/:pid

用途：移除一名成员，`ptype` 为 `user` / `group`。权限：Contributor+，空间 admin；key `docs_write`。

响应：204；移除最后一名管理员返回 400，成员不存在 404。

### PUT /api/v1/docs/spaces/:sid/knowledge-base

用途：绑定或解绑知识库与附件存储后端（前端没有入口）。只校验对象属于本工作区，不检查调用者能否编辑该知识库。权限：Contributor+，空间 admin；key `docs_write`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `knowledge_base_id` | string | 缺省不变，空字符串解绑 |
| `storage_backend_id` | string | 缺省不变，空字符串解绑 |

响应：200 `{"success":true,"data":{SpaceView}}`。改绑、解绑后的镜像迁移由后台同步完成，见功能页 6.1。

```bash
curl -X PUT $BASE/api/v1/docs/spaces/<sid>/knowledge-base -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"knowledge_base_id":"kb-1"}'
```

## 存储用量与配额

### GET /api/v1/docs/spaces/:sid/usage

用途：空间附件用量与配额，任何成员都能看。权限：Viewer+，空间 reader；key `docs_read`。

响应：200 `{"success":true,"data":{space_id,used_bytes,quota_bytes,from_default,can_manage}}`。`quota_bytes` 为 0 表示不限；`from_default` 表示数字来自部署默认值 `YUHENG_DOCS_SPACE_QUOTA_BYTES`。

### PUT /api/v1/docs/spaces/:sid/quota

用途：设置单个空间的附件配额。权限：**Admin+**，空间 reader，服务层再要求工作区管理员；key `docs_admin`。请求体：`{"quota_bytes":N}`，0 为不限，负数 400。可以设到低于当前用量，只阻止继续增长。

响应：200，返回与 `GET /usage` 相同的对象。

```bash
curl -X PUT $BASE/api/v1/docs/spaces/<sid>/quota -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"quota_bytes":1073741824}'
```

## 页面树与回收站

树节点（`TreeNode`）为页面行字段（`id`、`short_id`、`space_id`、`parent_id`、`position`、`title`、`icon`、`cover`、`exclude_from_knowledge`、`is_locked`、`owner_id`、`word_count`、`created_at`、`updated_at` 等）外加 `has_children`、`can_edit`、`restricted`。

### GET /api/v1/docs/spaces/:sid/tree

用途：按父节点懒加载页面树，只返回调用者可见的页面。权限：Viewer+，空间 reader；key `docs_read`。

| 查询参数 | 说明 |
| --- | --- |
| `parent` | 父页面 ID，省略为空间根 |
| `cursor` | 上一页的 `next_cursor` |
| `limit` | 默认 500，最多 2000 |

响应：200 `{"success":true,"data":{"items":[TreeNode],"next_cursor":"..."}}`

```bash
curl "$BASE/api/v1/docs/spaces/<sid>/tree?limit=100" -H "Authorization: Bearer $TOKEN"
```

### 回收站

| 方法与路径 | 权限 | 说明 |
| --- | --- | --- |
| `GET /api/v1/docs/spaces/:sid/trash` | Viewer+，空间 reader；key `docs_read` | 最近删除在前，只列删除根；每项为页面字段加 `deleted_by_user`、`can_restore` |
| `DELETE /api/v1/docs/spaces/:sid/trash` | Contributor+，空间 admin；key `docs_write` | 清空回收站，返回 `{"purged":N}` |
| `DELETE /api/v1/docs/spaces/:sid/trash/:pid` | Contributor+，空间 admin；key `docs_write` | 彻底删除一个已删除页面及其子树，返回 `{"purged":["<page_id>",...]}`；页面不在回收站返回 409 |

从回收站恢复页面见 `POST /pages/:pid/restore`。

## 页面（/api/v1/docs/pages）

页面对象（`PageView`）为页面行字段加 `role`、`can_edit`、`has_children`、`restricted`、`labels`、`favourite`、`steward_id`、`steward`、`can_change_owner`；页面行字段包括 `content`（ProseMirror JSON）、`ydoc_version`、`superseded_by`、`source_refs`、`knowledge_id`、`contributor_ids`、`creator_id`、`last_editor_id`、`content_updated_at` 等。

### POST /api/v1/docs/pages

用途：在父页面（或空间根）末尾新建页面。权限：Contributor+，成员，服务层要求父页面 writer（空间根则空间 writer）；key `docs_write`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `space_id` | string | 是 | 所属空间 |
| `parent_id` | string | 否 | 父页面，省略为空间根；必须在同一空间 |
| `title` | string | 否 | 单行，最多 500 字 |
| `icon` | string | 否 | 图标 |
| `content` | object | 否 | ProseMirror JSON 正文，最多 4 MiB |
| `markdown` | string | 否 | Markdown 正文，最多 4 MiB |
| `template_id` | string | 否 | 从模板开始 |

`content`、`markdown`、`template_id` 三选一，同时给返回 400。父页面被锁定时 403。

响应：201 `{"success":true,"data":{PageView}}`

```bash
curl -X POST $BASE/api/v1/docs/pages -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"space_id":"<sid>","title":"快速开始","markdown":"# 快速开始"}'
```

### GET /api/v1/docs/pages/:pid · GET /api/v1/docs/pages/by-short-id/:short

用途：按 ID 或短标识读取页面元数据与调用者的有效角色。权限：Viewer+，页面 reader；key `docs_read`。

响应：200 `{"success":true,"data":{PageView}}`；页面在回收站时 410（见通用约定）。

### PATCH /api/v1/docs/pages/:pid

用途：改标题、图标、封面；正文走 `PUT /content` 或协同。权限：Contributor+，页面 writer；key `docs_write`。字段 `title`、`icon`、`cover` 均可选，空字符串清除图标或封面；`cover` 最多 1024 字节。锁定页面 403。

响应：200 `{"success":true,"data":{PageView}}`

### POST /api/v1/docs/pages/:pid/move

用途：改父页面、排序位置或所属空间。跨空间时携带子树，调用者看不到的子页面留在原空间根下。权限：Contributor+，页面 writer；跨空间另需目标空间 writer；key `docs_write`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `parent_id` | string / null | 新父页面；null 或缺省为空间根 |
| `space_id` | string | 移动到另一个空间 |
| `after_id` | string / null | 排在这个兄弟之后；显式 null 排第一；缺省排最后 |
| `position` | string | 显式排序键，优先于 `after_id` |

响应：200 `{"success":true,"data":{"page":{...},"orphaned":[...],"rebalanced":bool}}`。自己做自己的父页面、`after_id` 不是目标位置的兄弟、目标父页面在别的空间返回 400；目标父页面锁定 403。

```bash
curl -X POST $BASE/api/v1/docs/pages/<pid>/move -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"parent_id":"<parent>","after_id":null}'
```

### POST /api/v1/docs/pages/:pid/duplicate

用途：复制页面及调用者可见的子树（一次最多 2000 页），副本继承目标位置的权限，副本之间的页面链接改指向副本。权限：Contributor+，源页面 reader，服务层要求目标位置 writer；key `docs_write`。

请求体可省略：`space_id`（复制到另一个空间，空为原位置之后）、`parent_id`（仅跨空间时有效，放到目标空间某页下）、`title`（覆盖副本标题）。

响应：201 `{"success":true,"data":{"page":{...},"child_ids":[...],"count":N}}`；子树超过 2000 页返回 400。

### DELETE /api/v1/docs/pages/:pid

用途：页面连同子树进入回收站，断开正在编辑的人、撤销指向它们的公开链接。权限：Contributor+，页面 writer；key `docs_write`。

响应：200 `{"success":true,"data":{"page_id":"...","deleted":N}}`；锁定页面 403。

### POST /api/v1/docs/pages/:pid/restore

用途：从回收站恢复页面及随它删除的子页面；父页面仍在回收站时挂到空间根下。权限：Contributor+，成员，服务层要求该页面 writer；key `docs_write`。

响应：200 `{"success":true,"data":{PageView}}`；页面不在回收站 409。

### 相邻页面与附件

| 方法与路径 | 权限 | 说明 |
| --- | --- | --- |
| `GET /api/v1/docs/pages/:pid/ancestors` | Viewer+，页面 reader；key `docs_read` | 从空间根到父页面的祖先数组，不含本页 |
| `GET /api/v1/docs/pages/:pid/children` | 同上 | 直接子页面，参数 `cursor`、`limit`（默认 500，最多 2000），返回 `{items:[TreeNode],next_cursor}` |
| `GET /api/v1/docs/pages/:pid/attachments` | 同上 | 本页附件列表，字段见[附件](#附件) |

## 页面正文与独占编辑

### GET /api/v1/docs/pages/:pid/content

用途：读取正文。权限：Viewer+，页面 reader；key `docs_read`。查询参数 `format`：`json`（默认）或 `html`，其它值 400。

响应：200 `{"success":true,"data":{"page_id":"...","ydoc_version":N,"content":{...},"html":"..."}}`，`html` 只在 `format=html` 时出现。

### PUT /api/v1/docs/pages/:pid/content

用途：整体替换正文。有协同服务时作为一次 Yjs 事务应用到正在打开的文档上；没有时写 JSON 并清空 Yjs 状态。审计原因固定记为 REST 写入，调用方不能指定。权限：Contributor+，页面 writer；key `docs_write`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `content` | object | ProseMirror JSON |
| `markdown` | string | Markdown |

二者互斥；都不给表示清空页面。响应：200 `{"success":true,"data":{"ydoc_version":N,"applied":bool}}`。锁定页面 403，正文超过 4 MiB 或结构不合法 400。

```bash
curl -X PUT $BASE/api/v1/docs/pages/<pid>/content -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"markdown":"# 标题\n\n正文"}'
```

### 独占编辑租约与 Yjs 状态

只用于没有协同服务的部署；部署了协同服务时这五个接口一律返回 409。租约 5 分钟，客户端每分钟续期。

| 方法与路径 | 权限 | 请求 | 响应 |
| --- | --- | --- | --- |
| `GET /api/v1/docs/pages/:pid/lease` | Viewer+，页面 reader；key `docs_read` | — | `{page_id,held,held_by_me,holder,expires_at,renew_after_seconds,ydoc_version}` |
| `POST /api/v1/docs/pages/:pid/lease` | Contributor+，页面 writer；key `docs_write` | `{"session_id":"..."}`（必填，≤64 字符，每个标签页一个） | 同上；同一 session 重复调用即续期，他人持有未过期时返回持有者信息 |
| `DELETE /api/v1/docs/pages/:pid/lease` | 同上 | `session_id` 放查询参数或请求体 | 204；只有持有该租约的 session 能释放 |
| `GET /api/v1/docs/pages/:pid/ydoc` | Viewer+，页面 reader；key `docs_read` | — | `{page_id,ydoc,content,ydoc_version}`，`ydoc` 为 base64；从未协同编辑过的页面只给 `content` |
| `PUT /api/v1/docs/pages/:pid/ydoc` | Contributor+，页面 writer；key `docs_write` | `{"session_id","base_version","ydoc","content"}`，`ydoc` 为完整 Yjs 状态的 base64，`content` 为对应的 ProseMirror JSON | `{ydoc_version,lease}`；租约已过期或被他人持有 409，`base_version` 落后 409，锁定 403，超过 `YUHENG_DOCS_MAX_YDOC_BYTES` 400 |

这些写操作不经过 `Idempotency-Key` 中间件：它们自带幂等键（session 与 base_version），又按定时器重复发送。

## 页面权限与锁定

### GET /api/v1/docs/pages/:pid/effective-permission

用途：调用者在本页的有效权限，供客户端决定显示哪些控件。权限：Viewer+，页面 reader；key `docs_read`。

响应：200 `{"success":true,"data":{page_id,role,can_edit,can_comment,can_manage_access,restricted,restricted_here}}`

### GET /api/v1/docs/pages/:pid/access

用途：权限面板：本页是否切断继承、被哪些上级收窄、授权名单。能打开页面即可看。权限：Viewer+，页面 reader；key `docs_read`。

响应：200 `{"success":true,"data":{page_id,restricted,inherited_from,grants,can_manage,space_default}}`。`inherited_from` 是收窄本页的受限祖先 `{id,short_id,title,visible}`，看不到的祖先不给标题；`grants` 每项为 `{principal_type,principal_id,role,effective,in_space,name,email,avatar,is_default_group,group_member_count,added_by,created_at}`，`effective` 是这条授权实际生效的角色，`in_space=false` 表示此人在空间里没有角色、授权不起作用。

### PUT /api/v1/docs/pages/:pid/access

用途：切断或恢复权限继承。首次限制时自动把页面作者加入名单；恢复继承会删除本页全部授权。限制一个已入库或已分享的页面会立即把它移出知识库并使公开链接失效。权限：Contributor+，页面 admin；key `docs_admin`。请求体：`{"restricted":true|false}`。

响应：200，返回与 `GET /access` 相同的对象。

```bash
curl -X PUT $BASE/api/v1/docs/pages/<pid>/access -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"restricted":true}'
```

### GET /api/v1/docs/pages/:pid/grants

用途：只取授权名单，与 `GET /access` 的 `grants` 相同；页面未受限时为空数组。权限：Viewer+，页面 reader；key `docs_read`。

### POST /api/v1/docs/pages/:pid/grants

用途：授权一个用户或组。授权只能收窄，不能提升。权限：Contributor+，页面 admin；key `docs_admin`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `principal_type` | string | `user` / `group` |
| `principal_id` | string | 用户 ID 或用户组 ID |
| `role` | string | `reader` / `writer` / `admin` |

响应：200，返回权限面板对象。页面未受限（需先 `PUT /access`）、名单超过 100 项、角色非法、用户不是活跃成员都返回 400；用户组不存在 404。

### DELETE /api/v1/docs/pages/:pid/grants/:ptype/:principal

用途：移除一条授权。权限：Contributor+，页面 admin；key `docs_admin`。

响应：200，返回权限面板对象；授权不存在 404。

### PUT /api/v1/docs/pages/:pid/lock

用途：锁定或解锁。锁定后除空间管理员外一律只读，正在编辑的会话立即断开。权限：Contributor+，页面 admin；key `docs_admin`。请求体：`{"locked":true|false}`。

响应：200 `{"success":true,"data":{PageView}}`

## 负责人、知识库与知识健康

| 方法与路径 | 权限 | 请求 | 响应与错误 |
| --- | --- | --- | --- |
| `PUT /api/v1/docs/pages/:pid/knowledge` | Contributor+，页面 writer；key `docs_write` | `{"excluded":true|false}`，true 表示不参与知识库 | `PageView`；锁定页面 403。不是权限，只决定是否出现在 AI 回答里 |
| `PUT /api/v1/docs/pages/:pid/owner` | Contributor+，页面 writer；key `docs_write` | `{"owner_id":"..."}`（必填） | `PageView`；服务层要求当前负责人或页面 admin（否则 403），新负责人必须是工作区成员且能编辑本页（否则 400）；锁定页面也可以转交 |
| `POST /api/v1/docs/pages/:pid/review` | Contributor+，页面 writer；key `docs_write` | 无 | `{"page_id":"..."}`；重新开始镜像条目的复核计时。页面不在知识库中 409 |
| `GET /api/v1/docs/pages/:pid/findings` | Viewer+，页面 reader；key `docs_read` | — | `{items,other_count}`：`items` 每项 `{id,type,severity,score,overlap_ratio,related_page,evidence,assignee,extra}`，只列另一方是调用者能打开的页面的问题，其余只计入 `other_count` |
| `POST /api/v1/docs/pages/:pid/supersede` | Contributor+，页面 writer；key `docs_write` | `{"finding_id":"..."}`（必填，本页的一个问题） | `{retired_knowledge_id,how}`：保留本页，把问题另一方的页面排除出知识库并标记为已被取代。需要能编辑两个页面；另一方不是页面 409；问题不存在 404 |

知识健康的检测与处理规则见[知识健康](../03-features/22-knowledge-health.md)。

### POST /api/v1/docs/pages/:pid/draft

用途：用空间绑定知识库里调用方点名的条目起草本页正文（前端没有入口）。使用该知识库配置的摘要模型；写入走「替换正文」路径（先留历史快照，可撤销），所用条目记在页面 `source_refs`。权限：Contributor+，页面 writer；key `docs_write`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `instruction` | string | 是 | 起草要求，截断到 2000 字 |
| `knowledge_ids` | string[] | 是 | 至少 1 条、最多 20 条，必须属于本空间绑定的知识库 |

响应：200 `{"success":true,"data":{"page_id":"...","source_ids":[...],"markdown":"...","warnings":[...]}}`。锁定页面、部署未提供模型桥接时 403；空间没有绑定知识库、条目数量不对、模型输出无法转换为文档时 400；条目不存在或不属于绑定的知识库 404。

```bash
curl -X POST $BASE/api/v1/docs/pages/<pid>/draft -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"instruction":"整理成部署指南","knowledge_ids":["k-1","k-2"]}'
```

### POST /api/v1/docs/spaces/:sid/reindex

用途：立即把空间里符合条件的页面重新同步到绑定的知识库，分页执行（日常编辑由后台队列自动同步，这个接口用于新绑定或批量权限变更之后）。权限：**Admin+**，空间 reader，服务层再要求空间 admin；key `docs_admin`。

| 查询参数 | 说明 |
| --- | --- |
| `after` | 上一次返回的 `next_cursor` |
| `limit` | 默认 50，最多 200 |

响应：200 `{"success":true,"data":{"results":[{page_id,indexed,removed,reason}],"next_cursor":"..."}}`。`reason` 为跳过原因：`restricted`、`trashed`、`excluded`、`empty` 等，见功能页 6.2。

```bash
curl -X POST "$BASE/api/v1/docs/spaces/<sid>/reindex?limit=200" -H "Authorization: Bearer $TOKEN"
```

## 版本历史

路由都挂在页面下，服务层另外核对版本属于该页面，不属于时 404。

| 方法与路径 | 权限 | 说明 |
| --- | --- | --- |
| `GET /api/v1/docs/pages/:pid/revisions` | Viewer+，页面 reader；key `docs_read` | 时间倒序，不含正文。参数 `cursor`、`limit`（默认与上限 100）。返回 `{items:[{id,page_id,version,title,icon,reason,editor_ids,editors,created_by,created_at,word_count}],next_cursor}` |
| `GET /api/v1/docs/pages/:pid/revisions/:rid` | 同上 | 一个版本，字段同上外加完整 `content` |
| `GET /api/v1/docs/pages/:pid/revisions/:rid/diff` | 同上 | 与 `?to=<rid>` 或（缺省时）当前正文比较，返回 `{from,to,lines,blocks,line_summary,block_summary}`：`lines` 为逐行文本差异，`blocks` 为按块 ID 的结构差异 |
| `POST /api/v1/docs/pages/:pid/revisions/:rid/restore` | Contributor+，页面 writer；key `docs_write` | 恢复到该版本，走「替换正文」路径，恢复前先为当前内容留快照。返回 `{ydoc_version,applied}`；锁定页面 403 |

```bash
curl "$BASE/api/v1/docs/pages/<pid>/revisions/<rid>/diff" -H "Authorization: Bearer $TOKEN"
```

## 评论

全部只要页面 **reader**（写评论也是）：评审不需要编辑权。工作区角色下限都是 Viewer。评论对象为 `{id,page_id,parent_id,body,anchor,quoted_text,placement,creator,creator_id,created_at,edited_at,resolved_at,resolved_by,resolved_user,replies,can_edit,can_delete,can_resolve}`。

### GET /api/v1/docs/pages/:pid/comments

用途：按时间正序列出讨论，回复嵌在各自线程下。权限：页面 reader；key `docs_read`。查询参数 `resolved=true` 时包含已解决的线程（默认不含）。

响应：200 `{"success":true,"data":{"items":[Comment],"open":N,"total":N}}`

### POST /api/v1/docs/pages/:pid/comments

用途：发表评论或回复。权限：页面 reader；key `docs_write`。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `body` | object | 是 | ProseMirror 文档（页面 schema 的子集） |
| `anchor` | object | 否 | 编辑器的 Yjs 相对位置；缺省为整页评论 |
| `quoted_text` | string | 否 | 选中的原文，位置失效后用来重新定位 |
| `parent_id` | string | 否 | 回复某个线程；回复只有一层，回复一条回复返回 400 |

响应：200 `{"success":true,"data":{Comment}}`；页面已有 1000 个讨论时 400。

```bash
curl -X POST $BASE/api/v1/docs/pages/<pid>/comments -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"这里需要补充示例"}]}]}}'
```

### 修改、解决与删除

| 方法与路径 | 权限 | 请求 | 说明 |
| --- | --- | --- | --- |
| `PATCH /api/v1/docs/pages/:pid/comments/:cid` | 页面 reader；key `docs_write` | `{"body":{...}}`（必填） | 只有作者能改，否则 403；不移动锚点 |
| `POST /api/v1/docs/pages/:pid/comments/:cid/resolve` | 同上 | `{"resolved":true|false}`，false 为重开 | 作者本人或页面 writer 可以解决；回复不能单独解决（400） |
| `DELETE /api/v1/docs/pages/:pid/comments/:cid` | 同上 | — | 作者本人或空间 admin；删除线程连同回复。返回 `{"deleted":true}` |

## 提及、关注与通知

| 方法与路径 | 权限 | 说明 |
| --- | --- | --- |
| `GET /api/v1/docs/pages/:pid/mention-candidates` | Viewer+，页面 reader；key `docs_read` | @提及候选：只含能读本页的人。参数 `q`（姓名或邮箱）、`limit`（最多 12）。返回 `[{user_id,username,email,avatar}]` |
| `GET /api/v1/docs/pages/:pid/watch` | 同上 | 关注状态 `{page_id,reason,muted,watched}` |
| `PUT /api/v1/docs/pages/:pid/watch` | Viewer+，页面 reader；key `docs_write` | `{"watching":true|false}`，返回关注状态 |
| `PUT /api/v1/docs/pages/:pid/mute` | 同上 | `{"muted":true|false}`：静音后保持关注但不再收到评论与更新通知，@提及照常送达；返回关注状态 |

### GET /api/v1/docs/notifications

用途：本人的站内通知，时间倒序。通知属于接收者，不受页面权限影响。权限：Viewer+，成员；key `docs_read`。

| 查询参数 | 说明 |
| --- | --- |
| `unread` | `true` 只看未读 |
| `cursor` | 上一页的 `next_cursor` |
| `limit` | 默认与上限 50 |

响应：200 `{"success":true,"data":{"items":[{id,kind,page_id,space_id,comment_id,actor_id,actor,payload,read_at,created_at}],"unread":N,"next_cursor":"..."}}`。`payload` 含页面标题、摘录等；合并规则见功能页 4.3。

### POST /api/v1/docs/notifications/read · POST /api/v1/docs/notifications/archive

用途：标记已读 / 归档（从列表移除但不删除记录）。权限：Viewer+，成员；key `docs_write`。请求体：`{"ids":["..."]}`，空数组表示全部。

响应：200 `{"success":true,"data":{"updated":N}}`

```bash
curl -X POST $BASE/api/v1/docs/notifications/read -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"ids":[]}'
```

## 收藏、标签与空间首页

标签对象：`{id,space_id,name,color,page_count}`。`name` 最多 32 字；`color` 取 `gray`（默认）、`red`、`orange`、`yellow`、`green`、`teal`、`blue`、`purple`、`pink`。每个空间最多 200 个标签，每个页面最多 20 个。

| 方法与路径 | 权限 | 请求 | 说明 |
| --- | --- | --- | --- |
| `GET /api/v1/docs/spaces/:sid/labels` | Viewer+，空间 reader；key `docs_read` | — | 标签列表，`page_count` 只计未删除页面 |
| `POST /api/v1/docs/spaces/:sid/labels` | Contributor+，空间 writer；key `docs_write` | `{"name","color"}` | 返回标签；同名 409，超过 200 个 400 |
| `PATCH /api/v1/docs/spaces/:sid/labels/:lid` | Contributor+，空间 writer；key `docs_write` | `{"name","color"}` | 改名或改色；同名 409 |
| `DELETE /api/v1/docs/spaces/:sid/labels/:lid` | Contributor+，空间 admin；key `docs_write` | — | 从所有页面上摘掉，返回 `{"deleted":true}` |
| `PUT /api/v1/docs/pages/:pid/labels` | Contributor+，页面 writer；key `docs_write` | `{"label_ids":[...]}` | 设置后的**全部**标签，不是追加；只能用本空间的标签（否则 400）。返回本页标签列表 |
| `GET /api/v1/docs/spaces/:sid/pages-by-label` | Viewer+，空间 reader；key `docs_read` | 查询参数 `labels`（逗号分隔，取交集）、`limit` | 同时带有全部这些标签的页面，`[TreeNode]` |
| `GET /api/v1/docs/spaces/:sid/home` | Viewer+，空间 reader；key `docs_read` | — | `{recently_edited,labels,favourites}`，最近编辑最多 20 个，均按可见性过滤 |
| `PUT /api/v1/docs/pages/:pid/favourite` | Viewer+，页面 reader；key `docs_write` | `{"favourite":true|false}` | 返回 `{"favourite":bool}` |
| `GET /api/v1/docs/favourites` | Viewer+，成员；key `docs_read` | — | 本人在所有空间的收藏，`[TreeNode]`，失去权限的页面不出现 |

没有 `/recent` 接口：「最近编辑」在空间首页里，「最近浏览」只存在浏览器本地。

## 链接、块引用与嵌入

这几个接口只要工作区成员，逐页按调用者权限过滤；看不到的页面与已删除的页面给出相同结果，不泄露存在性。

| 方法与路径 | 权限 | 请求 | 响应 |
| --- | --- | --- | --- |
| `GET /api/v1/docs/pages/:pid/backlinks` | Viewer+，页面 reader；key `docs_read` | — | 引用本页（页面链接或块引用）且调用者能打开的页面 `[PageRef]` |
| `GET /api/v1/docs/page-links/suggest` | Viewer+，成员；key `docs_read` | 查询参数 `q`（标题关键字）、`space`（留空为全部可见空间）、`limit`（最多 12） | `[PageRef]` |
| `POST /api/v1/docs/page-links/titles` | 同上 | `{"page_ids":[...]}`（必填，最多 200） | `[PageRef]`，看不到或已删除时 `resolved=false` |
| `POST /api/v1/docs/block-refs/resolve` | 同上 | `{"refs":[{"source_page_id","source_block_id"}]}`（必填，最多 200） | `[{source_page_id,source_block_id,state,content,title,icon,source_short_id}]`，`state` 为 `ok` / `missing`（块已删除或无权查看源页面）/ `pending`（源页面尚未保存过） |
| `GET /api/v1/docs/embeds/policy` | 同上 | — | `{providers,drawio_url}`：允许的嵌入服务商与 draw.io 地址（未配置为空） |
| `POST /api/v1/docs/embeds/resolve` | 同上 | `{"url":"..."}`（必填） | `{provider,url,embed_url,title,author,aspect_ratio}`；不在白名单或部署关闭了嵌入返回 400 |

`PageRef` 为 `{page_id,short_id,space_id,title,icon,breadcrumb,resolved}`。这三个 POST 只读不写，按读能力声明。

## 附件

附件对象：`{id,space_id,page_id,file_name,mime,size_bytes,kind,width,height,url,uploader,created_at,variants}`。

### POST /api/v1/docs/spaces/:sid/attachments

用途：上传附件（multipart）。按内容嗅探类型、SVG 去脚本、同一空间内容相同的文件只存一份，计入工作区与空间两层配额。权限：Contributor+，空间 writer；key `docs_write`。不经过 `Idempotency-Key` 中间件。

| 表单字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | 文件，上限 `YUHENG_DOCS_MAX_ATTACHMENT_BYTES`（默认 200 MiB） |
| `page_id` | 否 | 直接绑定到本空间的某个页面 |

响应：201 `{"success":true,"data":{Attachment}}`。空文件、超过大小、页面不在本空间 400；超出工作区或空间配额 409。

```bash
curl -X POST $BASE/api/v1/docs/spaces/<sid>/attachments -H "Authorization: Bearer $TOKEN" \
  -F file=@diagram.png -F page_id=<pid>
```

### GET /api/v1/docs/attachments/:aid · DELETE /api/v1/docs/attachments/:aid

- `GET`：下载。权限：Viewer+，成员，服务层每次重新检查对所属页面的读权限；key `docs_read`。查询参数 `w` 取 320 / 800 / 1600 时返回图片的缩小版本。响应为文件本身（或 302 到存储后端的短时效签名地址），带 `X-Content-Type-Options: nosniff`，除图片、音视频、PDF 外一律 `Content-Disposition: attachment`，可渲染的内容附沙箱 CSP。
- `DELETE`：删除附件记录，底层对象在最后一条引用它的记录删除时才真正删除。权限：Contributor+，成员，服务层要求所属页面（未绑定页面时为空间）writer；key `docs_write`。响应 204。

两者看不到时都返回 404。

## 搜索

### GET /api/v1/docs/search

用途：子串匹配页面标题与正文、评论、以及块引用显示的文字，结果逐页按调用者权限过滤；块引用的命中按源页面的权限判断。权限：Viewer+，成员；key `docs_read`。

| 查询参数 | 必填 | 说明 |
| --- | --- | --- |
| `q` | 是 | 1–128 字，超出部分截断；为空时返回空结果而不是报错 |
| `space` | 否 | 只搜一个空间 |
| `limit` | 否 | 最多 50 |

响应：200 `{"success":true,"data":{"query":"...","hits":[{kind,page_id,short_id,space_id,space_slug,title,excerpt,comment_id,source_page_id,score}],"truncated":bool}}`

```bash
curl "$BASE/api/v1/docs/search?q=部署&space=<sid>" -H "Authorization: Bearer $TOKEN"
```

## 模板

模板按自己的 ID 寻址（工作区模板不属于任何空间），守卫只要求工作区成员，范围在服务层判断：

| 范围 | 读取、使用 | 创建、修改 | 删除 |
| --- | --- | --- | --- |
| 空间模板（`space_id` 非空） | 该空间 reader | 该空间 writer | 该空间 **admin** |
| 工作区模板（`space_id` 为空） | 所有成员 | 工作区 Owner/Admin | 工作区 Owner/Admin |

模板对象：`{id,space_id,name,description,icon,category,shared,content,creator,can_edit,created_at,updated_at}`（列表不含 `content`）。

| 方法与路径 | 权限 | 请求 | 说明 |
| --- | --- | --- | --- |
| `GET /api/v1/docs/templates` | Viewer+，成员；key `docs_read` | 查询参数 `space`：给定时同时返回该空间的模板 | 工作区模板在前 |
| `GET /api/v1/docs/templates/:tid` | 同上 | — | 含正文 |
| `POST /api/v1/docs/templates` | Contributor+，成员；key `docs_write` | `{"space_id","name","description","icon","category","content","from_page_id"}`；`name` 必填，`content` 与 `from_page_id` 二选一 | 保存时去掉页面链接、块引用、@提及与附件；每个范围最多 200 个 |
| `PATCH /api/v1/docs/templates/:tid` | 同上 | 只传要改的字段：`name`、`description`、`icon`、`category`、`content` | 返回模板 |
| `DELETE /api/v1/docs/templates/:tid` | 同上 | — | 返回 `{"deleted":true}` |

```bash
curl -X POST $BASE/api/v1/docs/templates -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"space_id":"<sid>","name":"会议纪要","from_page_id":"<pid>"}'
```

## 导入与导出

### POST /api/v1/docs/spaces/:sid/imports

用途：导入单个 `.md` 或一个 `.zip`，按目录结构建页面树，后台任务。权限：Contributor+，空间 writer；key `docs_write`。不经过 `Idempotency-Key` 中间件。

| 表单字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | `.md` 或 `.zip`，上传包最多 256 MiB |
| `parent_id` | 否 | 导入到本空间某页之下，缺省为空间根 |

响应：202 `{"success":true,"data":{ImportJob}}`。页面不在本空间、包过大 400；部署没有配置导入所需的存储时 403。

```bash
curl -X POST $BASE/api/v1/docs/spaces/<sid>/imports -H "Authorization: Bearer $TOKEN" -F file=@docs.zip
```

### GET /api/v1/docs/imports/:jid

用途：查询导入任务。权限：Viewer+，成员，服务层要求能读目标空间；key `docs_read`。

响应：200 `{"success":true,"data":{id,space_id,kind,status,file_name,target_parent_id,created,attachments,skipped,error,done,created_at,finished_at}}`。`status` 为 `pending` / `running` / `succeeded` / `partial` / `failed`，`done=true` 表示已结束，`skipped` 列出跳过的文件与原因。

### POST /api/v1/docs/pages/:pid/export

用途：同步导出单页，直接返回文件。权限：Viewer+，页面 reader；key `docs_read`（导出不改变任何内容，虽然是 POST）。请求体可省略：`{"format":"markdown"|"html"}`（也接受 `md`、`htm`，默认 Markdown），其它值 400。

响应：200，文件本身（`Content-Disposition: attachment`，中文文件名用 RFC 5987 `filename*`）。

```bash
curl -X POST $BASE/api/v1/docs/pages/<pid>/export -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"format":"html"}' -OJ
```

### POST /api/v1/docs/spaces/:sid/export

用途：整个空间打包成 zip，后台任务；压缩包只含发起人当时能读的页面，最多 5000 页，24 小时后删除，附件不打包。权限：Viewer+，空间 reader；key `docs_read`。请求体同单页导出。

响应：202 `{"success":true,"data":{ExportJob}}`；部署没有配置导出存储时 403。

### GET /api/v1/docs/exports/:jid · GET /api/v1/docs/exports/:jid/download

- 查询任务：`{id,space_id,format,status,file_name,exported,skipped,error,ready,created_at,finished_at,expires_at}`，`ready=true` 时可下载。
- 下载：返回 zip 文件；尚未完成 409。

权限：Viewer+，成员；key `docs_read`。只有发起人能看到和下载自己的任务，其他人与过期任务都是 404。

## 公开链接（管理）

需要部署开启 `YUHENG_DOCS_PUBLIC_SHARING`，否则创建返回 403。受限页面不能分享（400）。链接对象：`{id,page_id,space_id,key,include_children,allow_search_index,has_password,expires_at,view_count,creator,created_at,live}`。

| 方法与路径 | 权限 | 请求 | 说明 |
| --- | --- | --- | --- |
| `GET /api/v1/docs/pages/:pid/shares` | Viewer+，页面 reader；key `docs_read` | — | 本页的公开链接，能读页面的人都能看 |
| `POST /api/v1/docs/pages/:pid/shares` | Contributor+，页面 writer；key `docs_write` | `{"include_children","allow_search_index","password","expires_at"}`，均可选；密码 4–64 字，过期时间最长一年 | 返回链接；每页最多 20 条有效链接 |
| `PATCH /api/v1/docs/pages/:pid/shares/:shid` | 同上 | 同上字段均可选，另有 `clear_expiry:true` 取消过期时间；`password` 传空字符串取消密码 | 改密码会让已解锁的访客重新输入 |
| `DELETE /api/v1/docs/pages/:pid/shares/:shid` | 同上 | — | 永久停用，返回 `{"revoked":true}` |

```bash
curl -X POST $BASE/api/v1/docs/pages/<pid>/shares -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"include_children":true,"password":"s3cret"}'
```

## 匿名访问（/api/v1/docs/public、/api/v1/docs/public-spaces）

注册在认证中间件之前，不需要也不接受登录态；只有 `YUHENG_DOCS_PUBLIC_SHARING=true` 时注册。链接 key 就是全部凭证。响应都带 `Cache-Control: private, no-store`（公开空间除外）与 `X-Robots-Tag`。

### GET /api/v1/docs/public/:key

用途：打开公开链接。查询参数 `page`：子树中某页的 `short_id`，缺省为链接根页。有密码的链接解锁后用请求头 `X-Docs-Share-Unlock: <unlock_token>` 再请求。

响应：总是 200 `{"success":true,"data":{"state":"...","page":{...}}}`，由 `state` 区分结果：`ok`（带 `page`）、`password`（需要密码）、`expired`、`revoked`、`gone`（页面已删除或已受限）。`page` 为 `{title,icon,html,short_id,updated_at,children,breadcrumb,allow_search_index,space_name}`，只有渲染后的正文，没有评论、历史和其他人的名字。`allow_search_index=true` 时 `X-Robots-Tag: index, follow`，否则 `noindex, nofollow, noarchive`。

```bash
curl $BASE/api/v1/docs/public/<key>
```

### POST /api/v1/docs/public/:key/unlock

用途：用密码解锁。请求体：`{"password":"..."}`。每 IP 每分钟 10 次（超限 429），每条链接 15 分钟内另有尝试上限。

响应：200 `{"success":true,"data":{"state":"ok","unlock_token":"..."}}`（12 小时有效）；链接失效时返回对应 `state`；密码错误 403。

```bash
curl -X POST $BASE/api/v1/docs/public/<key>/unlock -H 'Content-Type: application/json' -d '{"password":"s3cret"}'
```

### GET /api/v1/docs/public-spaces/:sid · GET /api/v1/docs/public-spaces/:sid/pages/:short

- 空间：`{id,name,slug,description,icon,pages}`，`pages` 为顶层页面。空间必须是 `public`，否则 404。用空间 ID 而不是 slug 寻址（slug 只在工作区内唯一，访客没有工作区）。
- 页面：按 `short_id` 返回一页，结构同公开链接的 `page`。受限页面与回收站页面一律 404。

两者都带 `X-Robots-Tag: index, follow`。页面里的附件、图片地址指向需要登录的 `/api/v1/docs/attachments/:aid`，匿名访客加载不到（见功能页第 9 节）。

## 维护

| 方法与路径 | 说明 |
| --- | --- |
| `POST /api/v1/docs/maintenance/orphan-attachments` | 清理上传超过 24 小时、未被任何页面引用的附件 |
| `POST /api/v1/docs/maintenance/expired-trash` | 清理超过 `YUHENG_DOCS_TRASH_RETENTION_DAYS` 的回收站页面，与手动清空回收站走同一路径 |

权限：Admin+，成员；key `docs_admin`。查询参数：`dry_run`（**默认试运行**，只有 `dry_run=false` 才真正删除）、`limit`（本次处理条数上限）。不经过 `Idempotency-Key` 中间件。后台清理器按 `YUHENG_DOCS_CLEANUP_INTERVAL_MINUTES` 定时执行同样的清理，这两个接口用于先看一眼或加速消化积压。

响应：200 `{"success":true,"data":{dry_run,considered,deleted,bytes_released,failed,more}}`

```bash
curl -X POST "$BASE/api/v1/docs/maintenance/expired-trash?dry_run=true" -H "Authorization: Bearer $TOKEN"
```

## 用户组（/api/v1/groups）

工作区级能力，在线文档是第一个使用者，随文档模块一起注册。每个工作区有一个默认组，所有成员隐式属于它：默认组不能改名、删除，也不能增删成员。API key 调用全部路由（包括读）都需要 `docs_admin`。用户组对象为 `{id,tenant_id,name,description,is_default,source,external_id,creator_id,created_at,updated_at,member_count}`。

| 方法与路径 | 权限 | 请求 | 说明 |
| --- | --- | --- | --- |
| `GET /api/v1/groups` | Viewer+，成员 | — | 默认组在前，附成员数 |
| `POST /api/v1/groups` | Admin+，成员 | `{"name","description","member_ids"}`，`name` 必填 | 201；组名在工作区内不区分大小写唯一，重名 409；默认组的名字保留（400）；`member_ids` 最多 200，必须是活跃成员 |
| `GET /api/v1/groups/:gid` | Viewer+，成员 | — | 单个用户组 |
| `PATCH /api/v1/groups/:gid` | Admin+，成员 | `{"name","description"}`，均可选 | 重名 409 |
| `DELETE /api/v1/groups/:gid` | Admin+，成员 | — | 204，同时移除该组在所有空间与页面上的授权 |
| `GET /api/v1/groups/:gid/members` | Viewer+，成员 | 查询参数 `q`（用户名/邮箱）、`page`（默认 1）、`page_size`（默认 20，最大 100） | `{members,total,page,page_size}` |
| `PUT /api/v1/groups/:gid/members` | Admin+，成员 | `{"user_ids":[...]}`（必填，最多 200） | 追加成员，已在组内的跳过；返回用户组对象 |
| `DELETE /api/v1/groups/:gid/members/:uid` | Admin+，成员 | — | 204；不在组内 404 |

```bash
curl -X POST $BASE/api/v1/groups -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"研发","member_ids":["u-1","u-2"]}'
```

## 协同服务回调（/internal/collab）

不在 `/api/v1` 下，也不是给客户端用的：这是协同服务（`collab/`）回调 app 的接口，只在模块开启且配置了 `YUHENG_COLLAB_SHARED_SECRET` 时注册，注册在会话中间件之前。每个请求都要带 `X-Collab-Timestamp` 与 `X-Collab-Signature`（对方法、路径、时间戳和请求体的 HMAC，时钟偏差不超过 5 分钟），签名不对返回 401。不要把这些路径暴露到公网，前端 nginx 只转发 `/collab` WebSocket。

| 方法与路径 | 用途 |
| --- | --- |
| `POST /internal/collab/authenticate` | 用浏览器带来的 `token` 解析连接者对 `page_id` 的权限，请求 `{token,page_id,tenant_id,connection_id}`，返回 `{user_id,display_name,avatar,access,ydoc_version,tenant_id,space_id}` |
| `GET /internal/collab/load/:pid?tenant=<id>` | 加载页面的 Yjs 状态（`application/octet-stream`，版本在 `X-YDoc-Version` 头）；从未协同编辑过的页面返回 `{content,ydoc_version}` |
| `POST /internal/collab/store` | 落盘合并后的文档，请求 `{tenant_id,page_id,base_version,ydoc,content,editor_ids,awareness_count}`，返回 `{ydoc_version}`；版本冲突 409（协同服务合并后重试），锁定等拒绝为 4xx `{"code":"rejected"}` |
| `GET /internal/collab/health` | 协同服务确认能连上 app，返回 `{"status":"ok"}` |
