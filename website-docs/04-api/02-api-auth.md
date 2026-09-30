# API 参考：认证与用户

路由注册：`internal/router/routes_auth_tenant.go` 的 `RegisterAuthRoutes` 与 `RegisterMyInvitationRoutes`，个人收藏在 `internal/router/routes_organization.go` 的 `RegisterUserFavoriteRoutes`。Handler：`internal/handler/auth.go`、`internal/handler/auth_register_by_invite.go`、`internal/handler/tenant_invitation.go`、`internal/handler/user_resource_favorite.go`。

除特别标注外，本组接口在认证中间件之后仅要求“已登录”（无角色下限），且只接受 JWT：API key 能调用的只有 `GET /auth/me`。免认证接口见各条目。

## 认证（/api/v1/auth）

### POST /api/v1/auth/register

用途：注册新用户（自助注册模式）。免认证，每 IP 10 次/分钟。Handler: `internal/handler/auth.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `username` | string | 是（`binding:"required,min=2,max=50"`） | 用户名，2-50 字符 |
| `email` | string | 是（`binding:"required,email"`） | 邮箱 |
| `password` | string | 是（`binding:"required,min=6"`） | 密码（≥6 位） |

新用户是否得到个人空间由服务端的注册策略决定，调用方不能在请求里指定。注册未开放（`invite_only`，或 `auto` 模式下已有用户）时返回 403。

注册模式按「系统设置 `auth.registration_mode` > 配置文件 > 默认 `auto`」解析。`auto` 只在系统还没有任何用户时开放，首个注册者成为系统管理员。环境变量 `DISABLE_REGISTRATION` 在启动时折算进配置：`true` 等于 `invite_only`（一直关闭），`false` 等于 `self_serve`（一直开放），不设则保持 `auto`。`/auth/register` 与 `/auth/config` 用同一套解析，界面与接口的判断总是一致。

响应：201 `{"success":true,"message":"Registration successful","user":{User}}`

```bash
curl -X POST $BASE/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"a@ex.com","password":"secret123"}'
```

### POST /api/v1/auth/register-by-invite

用途：通过邀请/分享链接 token 注册并加入空间。免认证，IP 限流 30 次/分钟。Handler: `internal/handler/auth_register_by_invite.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `token` | string | 是（`binding:"required"`） | 邀请 token |
| `email` | string | 是（`binding:"required,email"`） | 邮箱 |
| `username` | string | 是（`binding:"required"`） | 用户名 |
| `password` | string | 是（`binding:"required,min=6"`） | 密码（≥6 位） |

响应：201，同 Login（`user/active_tenant/memberships/token/refresh_token`）。

```bash
curl -X POST $BASE/api/v1/auth/register-by-invite -H 'Content-Type: application/json' \
  -d '{"token":"<invite_token>","email":"a@ex.com","username":"alice","password":"secret123"}'
```

### POST /api/v1/auth/invitations/lookup

用途：匿名查询邀请 token 对应的空间信息（注册前预览）。免认证，IP 限流。Handler: `internal/handler/auth_register_by_invite.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `token` | string | 是（`binding:"required"`） | 邀请 token |

响应：200 `{"success":true,"data":{"tenant_id","tenant_name","role","expires_at"}}`

```bash
curl -X POST $BASE/api/v1/auth/invitations/lookup -H 'Content-Type: application/json' -d '{"token":"<invite_token>"}'
```

### POST /api/v1/auth/login

用途：邮箱密码登录。免认证，每 IP 30 次/分钟，另有按账号的连续失败锁定。Handler: `internal/handler/auth.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `email` | string | 是（`binding:"required,email"`） | 邮箱 |
| `password` | string | 是（`binding:"required,min=6"`） | 密码 |

响应：200 `{"success":true,"user":{...},"active_tenant":{...},"memberships":[...],"token":"...","refresh_token":"..."}`

邮箱或密码错误返回 401。同一账号 15 分钟内连续失败 5 次后锁定 15 分钟，期间返回 429（code 1006）；有 Redis 时失败计数在实例间共享。

```bash
curl -X POST $BASE/api/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"a@ex.com","password":"secret123"}'
```

### GET /api/v1/auth/config

用途：查询注册模式等认证配置。免认证。Handler: `internal/handler/auth.go`

响应：200 `{"success":true,"registration_mode":"self_serve|invite_only","configured_registration_mode":"auto|self_serve|invite_only","registration_open":true,"first_user":false}`。`registration_mode` 是**当前生效**的状态（`auto` 且已有用户时读作 `invite_only`）；`first_user` 为 true 表示这是全新部署，下一个注册者将成为系统管理员

```bash
curl $BASE/api/v1/auth/config
```

### POST /api/v1/auth/switch-tenant

用途：切换当前活跃空间并换发 token。需登录（无空间也可调用），每 IP 60 次/分钟。Handler: `internal/handler/auth.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `tenant_id` | uint64 | 是（`binding:"required"`） | 目标空间 ID |
| `refresh_token` | string | 否 | 用于换发新 token |

响应：200，同 Login。

```bash
curl -X POST $BASE/api/v1/auth/switch-tenant -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"tenant_id":2}'
```

### GET /api/v1/auth/oidc/config

用途：查询 OIDC 是否启用及显示名。免认证。Handler: `internal/handler/auth.go`

响应：200 `{"success":true,"enabled":bool,"provider_display_name":"..."}`

```bash
curl $BASE/api/v1/auth/oidc/config
```

### GET /api/v1/auth/oidc/url

用途：获取 OIDC 授权跳转 URL。免认证。Handler: `internal/handler/auth.go`

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `redirect_uri` | string | 是 | 回调地址 |

响应：200 `{"success":true,"provider_display_name":"...","authorization_url":"...","state":"..."}`。nonce 不出现在响应里，而是写入 HttpOnly cookie，回调时校验。

```bash
curl "$BASE/api/v1/auth/oidc/url?redirect_uri=https://app.example.com/callback"
```

### GET /api/v1/auth/oidc/callback

用途：OIDC 授权回调（浏览器重定向进入）。免认证。Handler: `internal/handler/auth.go`

查询参数：`code`、`state`、`error`、`error_description`（均由 OIDC 提供方带回）。

响应：总是 302 重定向到前端 `/`，结果放在 URL hash 里：

- 成功：`#oidc_result=<base64url(JSON)>`，JSON 与登录响应同构（`success`、`user`、`active_tenant`、`memberships`、`token`、`refresh_token`，外加 `is_new_user`）。
- 失败：`#oidc_error=<原因>[&oidc_error_description=<说明>]`，原因为 IdP 返回的 `error`，或 `invalid_state`、`missing_code`、`login_failed`、`payload_encode_failed`。

```bash
curl -i "$BASE/api/v1/auth/oidc/callback?code=xxx&state=yyy"
```

### GET /api/v1/auth/oidc/start

用途：直接 302 跳转到 OIDC 提供方的授权页，不需要前端 JS 先取 URL。适合企业门户等外部平台放一个链接即可发起登录，借助 IdP 已有的 SSO 会话免再次输入密码。回调地址由请求自身的 scheme 与 host 推出（`/api/v1/auth/oidc/callback`）。免认证。Handler: `internal/handler/auth.go`

响应：302 重定向到授权 URL。

```bash
curl -i $BASE/api/v1/auth/oidc/start
```

### POST /api/v1/auth/refresh

用途：用 refresh token 换发新 token。免认证。Handler: `internal/handler/auth.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `refreshToken` | string | 是（`binding:"required"`） | refresh token |

响应：200 `{"success":true,"message":"Token refreshed successfully","access_token":"...","refresh_token":"..."}`。注意请求字段是驼峰 `refreshToken`，响应字段是下划线。

```bash
curl -X POST $BASE/api/v1/auth/refresh -H 'Content-Type: application/json' -d '{"refreshToken":"<rt>"}'
```

### GET /api/v1/auth/validate

用途：校验当前 token 是否有效。需登录（无空间可调用）。Handler: `internal/handler/auth.go`

响应：200 `{"success":true,"message":"Token is valid","user":{UserInfo}}`

```bash
curl $BASE/api/v1/auth/validate -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/auth/logout

用途：登出（失效当前 token）。需登录。无请求体。Handler: `internal/handler/auth.go`

响应：200 `{"success":true,"message":"Logout successful"}`

```bash
curl -X POST $BASE/api/v1/auth/logout -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/auth/me

用途：查询当前调用者身份（用户/空间/成员关系/能力）。需登录；API key 亦可（策略 `apiKeyAny()`，任何有效 key）。Handler: `internal/handler/auth.go`

响应：200 `{"success":true,"data":{"user":{UserInfo},"tenant":{TenantResponse},"memberships":[...],"tenant_required":bool,"capabilities":{"can_create_tenant":bool,"auto_accept_invitation":bool}}}`。`tenant_required` 为 true 表示调用者还没有任何可用空间。

```bash
curl $BASE/api/v1/auth/me -H "X-API-Key: $API_KEY"
```

### PUT /api/v1/auth/me/preferences

用途：更新个人偏好（最近活跃空间）。需登录。Handler: `internal/handler/auth.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `last_active_tenant_id` | *uint64 | 否 | 最近活跃空间 ID，null 清除 |

响应：200 `{"success":true,"data":{UserPreferences}}`

```bash
curl -X PUT $BASE/api/v1/auth/me/preferences -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"last_active_tenant_id":2}'
```

### POST /api/v1/auth/change-password

用途：修改密码。需登录。Handler: `internal/handler/auth.go`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `old_password` | string | 是（`binding:"required"`） | 旧密码 |
| `new_password` | string | 是（`binding:"required"`） | 新密码：8-32 个字符，至少含一个字母和一个数字，且不能与旧密码相同 |

响应：200 `{"success":true,"message":"Password changed successfully"}`

成功后该用户的全部会话（token）被吊销，需要用新密码重新登录。违反密码策略或新旧相同返回 1010（`details` 分别为 `password_policy` / `same_password`），旧密码错误返回 1000。

```bash
curl -X POST $BASE/api/v1/auth/change-password -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"old_password":"old","new_password":"newpass1"}'
```

## 我的邀请（/api/v1/me/invitations）

服务层保证“仅被邀请人可接受/拒绝”；无角色下限（无空间的新用户也可用）。Handler: `internal/handler/tenant_invitation.go`

### GET /api/v1/me/invitations

用途：列出发给我的邀请。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `include_terminal` | bool | 否 | `true` 时包含已完结的邀请 |

响应：200 `{"success":true,"data":{"invitations":[TenantInvitationResponse],"total":N}}`

```bash
curl $BASE/api/v1/me/invitations -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/me/invitations/pending-count

用途：待处理邀请计数（轻量轮询）。

响应：200 `{"success":true,"data":{"pending_count":N}}`

```bash
curl $BASE/api/v1/me/invitations/pending-count -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/me/invitations/:inv_id/accept

用途：接受邀请，写入成员关系。路径参数：`inv_id` 邀请 ID。无请求体。

响应：200 `{"success":true,"data":{"membership":{"tenant_id","role","status","joined_at"}}}`

```bash
curl -X POST $BASE/api/v1/me/invitations/12/accept -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/me/invitations/:inv_id/decline

用途：拒绝邀请。路径参数：`inv_id`。无请求体。

响应：200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/me/invitations/12/decline -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/me/invitations/accept-by-token

用途：已登录用户用共享邀请链接的 token 加入空间（与 `register-by-invite` 相对，不创建新账号）；对已是成员的用户幂等。调用者还没有默认空间时，加入的空间会成为其默认空间。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `token` | string | 是（`binding:"required"`） | 邀请链接 token |

响应：200 `{"success":true,"data":{"membership":{"tenant_id","role","status","joined_at"},"tenant_name":"..."}}`；链接无效、过期或已撤销返回 410。

```bash
curl -X POST $BASE/api/v1/me/invitations/accept-by-token -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"token":"<invite_token>"}'
```

## 个人收藏（/api/v1/user/favorites）

收藏是个人导航用的书签，只作用于调用者自己在当前空间里的收藏，不能查看或修改别人的收藏。Viewer+，仅 JWT（未对 API key 声明）。资源类型：`kb`（知识库）、`doc_space`（在线文档空间）、`doc_page`（在线文档页面）。Handler: `internal/handler/user_resource_favorite.go`

| 方法与路径 | 说明 |
| --- | --- |
| `GET /user/favorites?type=kb` | 列出当前空间里收藏的该类资源，`type` 必填，非法值返回 400；每项含 `resource_type`、`resource_id`、`created_at` |
| `POST /user/favorites` | 收藏，请求体 `{"type":"kb","id":"<kb_id>"}` |
| `DELETE /user/favorites/:type/:id` | 取消收藏 |

响应：列表为 `{"success":true,"data":[...]}`，增删为 `{"success":true}`。

```bash
curl -X POST $BASE/api/v1/user/favorites -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"type":"kb","id":"<kb_id>"}'
```
