# API 参考：文件服务

路由注册：`internal/router/files.go` 的 `serveFilesWithResources`、`servePresignedFiles`、`servePresignedPreview`。其中 `/files` 挂在全局认证之后，`/api/v1/files/presigned` 与 `/r/:token` 免认证（验签或 token 即能力凭证）。

## 文件服务

### GET /files

用途：认证后的统一文件代理（本地 / S3 兼容存储）。权限：任意已认证空间成员；API key 需非 KB 受限（full-access 或全空间 retrieve，`middleware.AllowFileServeAPIKey()`）；路径强制同空间（`ValidateStoragePathTenant`）。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file_path` | string | 是 | `provider://...` 路径（禁止 `..`；跨空间 403） |

响应：200 文件流（`X-Content-Type-Options: nosniff`；非白名单类型强制 `Content-Disposition: attachment`）。

```bash
curl "$BASE/files?file_path=local://1/docs/a.png" -H "Authorization: Bearer $TOKEN" -o a.png
```

### GET|HEAD /api/v1/files/presigned

用途：HMAC 签名 URL 文件访问（免认证，验签+过期校验，`SYSTEM_AES_KEY` 参与签名）。

| 查询参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file_path` | string | 是 | 存储路径 |
| `tenant_id` | uint64 | 是 | 空间 ID |
| `expires` | string | 是 | Unix 过期时间 |
| `sig` | string | 是 | HMAC 签名 |

响应：200 文件流（HEAD 仅返回头）；签名无效/过期 403。

```bash
curl "$BASE/api/v1/files/presigned?file_path=local://1/x.png&tenant_id=1&expires=1790000000&sig=abc" -o x.png
```

### GET /api/v1/files/presigned-preview

用途：诊断端点：返回给定路径将生成的预签名 HTTP URL。权限：Admin+，显式拒绝 API key（`DenyAPIKeyPrincipal`）。查询参数：`file_path`（必填）。

响应：200 `{"file_path","provider","url","rewritten":bool,"hint"}`

```bash
curl "$BASE/api/v1/files/presigned-preview?file_path=local://1/x.png" -H "Authorization: Bearer $TOKEN"
```

### GET|HEAD /r/:token

用途：短时效资源授权 URL（无法携带认证头的客户端）。免认证，token 即能力凭证；无效/过期 404。

响应：200 文件流（`Cache-Control: private, max-age=300`）。

```bash
curl $BASE/r/abc123 -o file.png
```
