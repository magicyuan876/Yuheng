# API 参考：文件服务

路由注册：`internal/router/files.go` 的 `serveFiles`、`serveKBScopedFiles`、`serveMessageScopedFiles`、`servePresignedPreview`、`serveResourceGrants`。其中 `/files` 与 `/api/v1/files/presigned-preview` 挂在引擎根上、全局认证之后，不经过 `/api/v1` 的 API key 网关，各自带独立的 API key 校验；`/r/:token` 免认证（token 本身就是凭据）。

各种 URL 形式怎么选、图片不显示怎么排查，见[图片与文件的对外访问](../03-features/21-file-access.md)。

**所有代理只接受 `resource://<handle>` 引用。** 引用对应一条资源记录，记录里写着文件所在的存储后端和后端内的位置，服务端按它读取文件——与调用者的空间、知识库当前绑定的后端都无关。存储自己的位置（`local://…`、`s3://…`）不是引用，传进来返回 400。

所有文件流响应都带 `X-Content-Type-Options: nosniff`；不在内联白名单里的类型强制 `Content-Disposition: attachment`，内容类型取自资源记录里的文件名与 MIME 类型。`file_path` 缺失或不是 `resource://` 引用返回 400，引用不存在（或文件已删除）返回 404。

## 认证后的代理

### GET /files

用途：空间范围的文件代理。权限：任意已认证空间成员；API key 需不受 KB 白名单限制（full-access 或全空间 `retrieve`，`middleware.AllowFileServeAPIKey()`）；资源必须属于调用者空间，否则 403。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file_path` | string | 是 | `resource://<handle>` |

响应：200 文件流（`Cache-Control: public, max-age=86400`）。

```bash
curl "$BASE/files?file_path=resource://AbCdEfGhIjKlMnOpQrStUv" -H "Authorization: Bearer $TOKEN" -o a.png
```

### GET /api/v1/knowledge-bases/:id/files

用途：知识库范围的文件代理，用来渲染知识库内容（分块、Wiki 页面）里嵌入的图片。先过 KB 访问守卫（知识库须属于本空间，否则 404）；资源须属于本空间，且位于 `exports/` 区域（正文图片与导出产物写在这里）——这条路由只渲染内容，不给原始上传文件（那走 `GET /knowledge/:id/download` 及其更严的权限）。权限：Viewer+；API key 需 `retrieve`/full，且不受 KB 白名单限制。查询参数：`file_path`（必填，`resource://`）。

响应：200 文件流（`Cache-Control: private, max-age=86400`）。

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/files?file_path=resource://AbCdEfGhIjKlMnOpQrStUv" \
  -H "Authorization: Bearer $TOKEN" -o chart.png
```

### GET /api/v1/sessions/:id/messages/:message_id/files

用途：消息范围的文件代理：回答里引用的资源按已持久化的消息本身授权——消息引用了它，调用者能读这条消息，就能读它；不接受客户端指定来源空间。消息不属于调用者的会话时返回 404。权限：Viewer+；API key `chat`/full。查询参数：`file_path`（必填，`resource://`）。

响应：200 文件流（`Cache-Control: private, max-age=86400`）。

## 免认证访问

### GET|HEAD /r/:token

用途：短时效资源授权 URL，给无法携带认证头的客户端使用（例如 `resource_urls=public` 时的外链）。token 即能力凭证，有效期 2 小时；无效或过期返回 404。对任何存储后端都有效。

响应：200 文件流（`Cache-Control: private, max-age=300`；HEAD 只返回头，但同样确认文件存在）。

```bash
curl $BASE/r/abc123 -o file.png
```

## 诊断

### GET /api/v1/files/presigned-preview

用途：返回 `resource_urls=public` 时会为给定资源生成的外链，用来检查外部可达性。权限：Admin+；显式拒绝 API key（`DenyAPIKeyPrincipal`）。查询参数：`file_path`（必填，本空间的 `resource://` 引用）。

响应：200 `{"file_path","storage_backend_id","url","rewritten":bool,"hint"}`。`rewritten=false` 表示没有可用的外链：本机目录后端在没设 `APP_EXTERNAL_URL` 时就是这样，`hint` 会说明。

```bash
curl "$BASE/api/v1/files/presigned-preview?file_path=resource://AbCdEfGhIjKlMnOpQrStUv" \
  -H "Authorization: Bearer $TOKEN"
```
