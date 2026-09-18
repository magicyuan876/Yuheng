# Yuheng 协同编辑服务（collab）

在线文档模块的实时协同边车：接收浏览器的 Yjs 同步 WebSocket，合并多人的
编辑并中继光标（Awareness），按防抖策略把文档状态交给 Go 服务持久化。它
没有数据库、没有用户表，所有业务判断（谁能读、谁能写、存到哪里）都由
Go 服务通过三条签名回调决定。

基于 [Hocuspocus](https://github.com/ueberdosis/hocuspocus)（MIT）与
[Yjs](https://github.com/yjs/yjs)（MIT）。本目录代码为 Yuheng 独立实现，MIT 许可。

## 与 Go 服务的契约

| 方向 | 路径 | 用途 |
|---|---|---|
| collab → Go | `POST /internal/collab/authenticate` | 校验用户 token，返回 `readwrite` / `readonly` |
| collab → Go | `GET /internal/collab/load/{page_id}?tenant=` | 取 Yjs 状态（或首次的 JSON 正文） |
| collab → Go | `POST /internal/collab/store` | 持久化：Yjs 状态 + JSON 投影 + 编辑者，乐观并发 |
| collab → Go | `GET /internal/collab/health` | 健康探测 |
| Go → collab | `POST /internal/collab/replace/{page_id}` | 用 JSON 一次性替换在线文档（导入 / 恢复 / AI 回写） |
| Go → collab | `DELETE /internal/collab/{page_id}` | 踢出连接并清缓存（删除页面 / 权限收窄） |

所有内部请求带 `X-Collab-Timestamp` 与 `X-Collab-Signature`（HMAC-SHA256，
见 `src/signing.ts` 与 Go 侧 `internal/docs/collab/sign.go`），时间戳误差
不超过 5 分钟。

浏览器连接：`ws://<collab>/<page_id>?tenant=<tenant_id>`，token 通过
Hocuspocus 的认证消息传递（`HocuspocusProvider` 的 `token` 选项）。

## 配置（环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `COLLAB_PORT` | `1234` | 监听端口 |
| `COLLAB_ADDRESS` | `0.0.0.0` | 监听地址 |
| `COLLAB_BACKEND_URL` | 必填 | Go 服务地址，如 `http://app:8080` |
| `COLLAB_SHARED_SECRET` | 必填 | 与 `YUHENG_COLLAB_SHARED_SECRET` 相同 |
| `COLLAB_REDIS_URL` | 空 | 多实例时必填，如 `redis://redis:6379/3` |
| `COLLAB_STORE_DEBOUNCE_MS` | `2000` | 最后一次编辑后多久持久化 |
| `COLLAB_STORE_MAX_WAIT_MS` | `10000` | 持续编辑时最长多久必须持久化一次 |
| `COLLAB_MAX_YDOC_BYTES` | `20971520` | 单页 Yjs 状态上限，与 Go 侧一致 |
| `COLLAB_AUTH_CACHE_MS` | `30000` | 鉴权结果缓存 |
| `COLLAB_RECHECK_INTERVAL_MS` | `300000` | 每条连接重新复核权限的间隔 |
| `COLLAB_UPDATES_PER_SECOND` | `200` | 单连接每秒允许的更新数，超过即断开 |
| `COLLAB_STORE_RETRY_BASE_MS` / `_MAX_MS` | `1000` / `30000` | Go 不可达时的重试退避 |
| `COLLAB_SHUTDOWN_FLUSH_TIMEOUT_MS` | `15000` | 停机时等待未持久化文档的时长 |
| `COLLAB_BACKEND_TIMEOUT_MS` | `10000` | 单次回调超时 |
| `COLLAB_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

## 故障行为

- Go 不可达：拒绝新连接（关闭码 1013），已连接会话继续在内存中合并，
  持久化按指数退避重试，文档不会被卸载；进程收到 SIGTERM 时先全量 flush。
- 持久化返回 409：拉取最新状态合并进在线文档后立即重试，Yjs 合并幂等。
- 只读连接发来的写入被丢弃并计数（`collab_readonly_updates_dropped_total`）。
- 超过大小上限的文档不会写入，客户端收到 `yuheng.error` 无状态消息。

## 端点

- `GET /healthz`：能连通 Go 的鉴权接口时返回 200，否则 503。
- `GET /metrics`：Prometheus 文本格式。

## 开发

```bash
npm ci
npm test            # node:test，内置一个假的 Go 服务端
npm run type-check
npm run build       # esbuild → dist/main.js
COLLAB_BACKEND_URL=http://localhost:8080 COLLAB_SHARED_SECRET=... npm run dev
```

镜像构建以仓库根目录为上下文：`docker build -f collab/Dockerfile .`。
