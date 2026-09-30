# API 参考：文件服务

路由注册：`internal/router/files.go` 的 `serveFilesWithResources`、`serveKBScopedFiles`、`serveMessageScopedFiles`、`servePresignedFiles`、`servePresignedPreview`、`serveResourceGrants`。其中 `/files` 与 `/api/v1/files/presigned-preview` 挂在引擎根上、全局认证之后，不经过 `/api/v1` 的 API key 网关，各自带独立的 API key 校验；`/api/v1/files/presigned` 与 `/r/:token` 免认证（签名或 token 本身就是凭据）。

各种 URL 形式怎么选、图片不显示怎么排查，见[图片与文件的对外访问](../03-features/21-file-access.md)。

所有文件流响应都带 `X-Content-Type-Options: nosniff`；不在内联白名单里的类型强制 `Content-Disposition: attachment`。`file_path` 缺失返回 400，含 `..` 返回 400。

## 认证后的代理

### GET /files

用途：空间范围的统一文件代理（本地 / S3 兼容存储）。权限：任意已认证空间成员；API key 需不受 KB 白名单限制（full-access 或全空间 `retrieve`，`middleware.AllowFileServeAPIKey()`）；路径必须属于调用者空间（`ValidateStoragePathTenant`），否则 403。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file_path` | string | 是 | `provider://...` 路径，或 `backend://<id>/provider://...` 指定存储后端实例 |

响应：200 文件流（`Cache-Control: public, max-age=86400`）。

```bash
curl "$BASE/files?file_path=local://1/docs/a.png" -H "Authorization: Bearer $TOKEN" -o a.png
```

### GET /api/v1/knowledge-bases/:id/files

用途：知识库范围的文件代理，用来渲染组织共享知识库内容里的图片（`/files` 只允许本空间路径，取不到属主空间的对象）。KB 访问守卫会把请求的空间改写为 KB 的属主空间，路径仍须属于该属主空间。权限：Viewer+，KB read；API key 需 `retrieve`/full，且不受 KB 白名单限制。查询参数：`file_path`（必填）。

响应：200 文件流（`Cache-Control: private, max-age=86400`）。

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/files?file_path=local://1/exports/chart.png" \
  -H "Authorization: Bearer $TOKEN" -o chart.png
```

### GET /api/v1/sessions/:id/messages/:message_id/files

用途：消息范围的文件代理：回答里引用的资源可能存放在另一个空间（例如引用了组织共享知识库的内容），授权依据是已持久化的消息本身，不接受客户端指定来源空间。消息不属于调用者的会话时返回 404。权限：Viewer+；API key `chat`/full。查询参数：`file_path`（必填）。

响应：200 文件流（`Cache-Control: private, max-age=86400`）。

## 免认证访问

### GET|HEAD /api/v1/files/presigned

用途：HMAC 签名 URL 文件访问，校验签名与过期时间（签名密钥由 `SYSTEM_AES_KEY` 派生，轮换后旧链接失效）。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file_path` | string | 是 | 存储路径 |
| `tenant_id` | uint64 | 是 | 空间 ID |
| `expires` | string | 是 | Unix 过期时间 |
| `sig` | string | 是 | HMAC 签名 |

响应：200 文件流（`Cache-Control: public, max-age=86400`；HEAD 只返回头，但同样确认对象存在）；参数缺失 400，签名无效或过期 403，对象不存在 404。

```bash
curl "$BASE/api/v1/files/presigned?file_path=local://1/x.png&tenant_id=1&expires=1790000000&sig=abc" -o x.png
```

### GET|HEAD /r/:token

用途：短时效资源授权 URL，给无法携带认证头的客户端使用。token 即能力凭证；无效或过期返回 404。

响应：200 文件流（`Cache-Control: private, max-age=300`）。

```bash
curl $BASE/r/abc123 -o file.png
```

## 诊断

### GET /api/v1/files/presigned-preview

用途：返回按当前空间存储配置会为给定路径生成的 HTTP URL，用来检查外部可达性。权限：Admin+；显式拒绝 API key（`DenyAPIKeyPrincipal`）。查询参数：`file_path`（必填）。

响应：200 `{"file_path","provider","url","rewritten":bool,"hint"}`。`rewritten=false` 表示 URL 没有被改写，本地存储通常是因为没设 `APP_EXTERNAL_URL`。存储配置不完整时返回 400 并带 `hint`。

```bash
curl "$BASE/api/v1/files/presigned-preview?file_path=local://1/x.png" -H "Authorization: Bearer $TOKEN"
```
