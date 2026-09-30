# 开发指南

本章面向准备对 Yuheng 做二次开发的工程师，介绍本地开发环境搭建、Makefile 命令、开发模式（`docker-compose.dev.yml`）、测试体系、代码规范、CI 与调试技巧。

## 1. 技术栈与环境要求

| 组件 | 目录 | 语言 / 运行时 | 版本要求（来源） |
| --- | --- | --- | --- |
| 主后端 `app` | `cmd/server` + `internal/` | Go | **Go 1.26.0**（`go.mod`），需 CGO（DuckDB 绑定） |
| 文档解析服务 `docreader` | `docreader/` | Python + gRPC | **Python >= 3.10.18**（`docreader/pyproject.toml`），依赖用 **uv** 管理（`uv.lock`） |
| 前端 `frontend` | `frontend/` | Node.js + Vue 3.5 | TypeScript ~6.0、Vite 7、Pinia、vue-router、vue-i18n；新代码用 Tailwind v4 + shadcn-vue（`src/components/ui/`），旧页面仍是 TDesign + Less，按文件逐个迁移。本地 Node 22 起（`@tsconfig/node22`），CI 用 Node 24 |
| 协同编辑服务 `collab` | `collab/` | Node.js + Yjs | `engines.node >= 22`；只在在线文档启用时需要 |
| CLI | `cli/`（独立 Go module） | Go | Go 1.26 |
| MCP Server | `mcp-server/` | Python | Python 3.10–3.13（CI 矩阵） |

还需要：

- **Docker**（带 Compose v2 插件）：开发模式的依赖服务跑在容器里；**`go test ./...` 也需要 Docker**，因为需要数据库的测试会启动真实的 ParadeDB 容器（见 4.1）；
- 数据库迁移 CLI（`scripts/migrate.sh` 依赖）：`go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`；
- `golangci-lint` v2（`make lint`）与 `gofumpt`（`make fmt` 调用 `scripts/gofumpt-tree.sh`，脚本固定了与 golangci-lint 内嵌版本一致的 gofumpt）；
- `swag`（`make install-swagger`，`make docs` 用）；
- `uv`（docreader 与 mcp-server 的依赖管理）。

## 2. 快速开始：开发模式

开发模式的思路是：**基础设施跑在 Docker 里，`app` 与 `frontend` 跑在本地**，改代码即时重启，无需反复构建镜像。入口是 `scripts/dev.sh`（Makefile 的 `dev-*` 目标是它的包装）。

```bash
# 1. 准备环境变量：dev.sh 加载 .env（必须存在），再用 .env.local 覆盖（可选）
cp .env.example .env

# 2. 启动依赖服务（postgres、redis、docreader、rustfs，默认还带 Langfuse）
make dev-start                      # 等价 ./scripts/dev.sh start
make dev-start DEV_ARGS=--neo4j     # 附加可选 profile
make dev-start DEV_ARGS=--docs      # 附加在线文档的协同服务

# 3. 另开终端：本地跑后端（go run ./cmd/server，带版本 ldflags）
make dev-app

# 4. 再开终端：本地跑前端（npm install && npm run dev）
make dev-frontend

# 其他
make dev-status   # 查看容器状态
make dev-logs     # 查看日志
make dev-stop     # 停止
make dev-restart  # 重启
```

开发模式下的访问地址：前端 `http://localhost:5173`、后端 API `http://localhost:8080`、PostgreSQL `localhost:5432`、Redis `localhost:6379`、RustFS 控制台 `http://localhost:9001`、Langfuse `http://localhost:3000`、Neo4j Browser `http://localhost:7474`（开了 `--neo4j` 时）。

也可以用 `./scripts/quick-dev.sh` 一次启动：它启动依赖服务后把后端与前端放到后台运行，日志写到 `logs/backend.log`、`logs/frontend.log`，进程号写到 `tmp/*.pid`。

`dev.sh app` 会先加载 `.env` / `.env.local`，再把数据库、Redis、docreader、RustFS、Neo4j 的地址改成 `127.0.0.1` 上的容器端口，所以 `.env` 里保留 compose 用的服务名也没关系。依赖服务跑在另一台机器上时，设 `DEV_REMOTE_HOST=<主机>`：`dev.sh start` 不再启动本地容器，`dev.sh app` 把这些地址指向该主机。

前端 dev server 监听 `5173`，把 `/api` 与 `/files` 代理到后端（`frontend/vite.config.ts` 的 `DEV_PROXY_TARGET`，取 `VITE_DEV_PROXY_TARGET` 或 `FRONTEND_BACKEND_URL`，默认 `http://localhost:8080`）。`vite preview`（端口 `4173`）用生产构建产物起服务，最接近 release 镜像。

### 2.1 docker-compose.dev.yml 服务清单

`docker-compose.dev.yml` 只包含依赖服务，不含 `app` / `frontend`：

| 服务 | 镜像 | 端口（默认） | 启动条件 |
| --- | --- | --- | --- |
| `postgres` | `paradedb/paradedb:v0.22.2-pg17`（自带 pgvector 与 pg_search） | `5432` | 默认 |
| `redis` | `redis:7.0-alpine`（`--requirepass`） | `6379` | 默认 |
| `docreader` | 本地构建 `docker/Dockerfile.docreader` | `50051`（gRPC） | 默认 |
| `rustfs` | `rustfs/rustfs`（按 digest 固定） | `9000` / 控制台 `9001` | 默认 |
| `langfuse-web` / `langfuse-worker` / `langfuse-clickhouse` / `langfuse-minio` / `langfuse-db-init` | Langfuse v3 自建栈，复用 dev 的 postgres（独立 `langfuse` 库）与 redis | web `3000` | 默认开启，`--no-langfuse` 关闭 |
| `collab` | 本地构建 `collab/Dockerfile` | `1234` | `--docs`；回调地址默认 `http://host.docker.internal:8080`，即本地跑的 app |
| `neo4j` | `neo4j:latest`（APOC 插件） | `7474` / `7687` | `--neo4j` / `--full` |
| `dex` | `dexidp/dex:latest`（配置 `misc/dex-config.yaml`） | `5556` | `--dex` / `--full` |
| `searxng`（+`searxng-init`） | `searxng/searxng:latest` | `127.0.0.1:8888` | `--full`（compose profile `searxng` / `full`） |
| `odl-hybrid` | 本地构建 `docker/Dockerfile.odl-hybrid` | `5002` | `--odl-hybrid`（镜像较大，不包含在 `--full` 里） |

`dev.sh start` 的参数：`--docs`、`--neo4j`、`--dex`、`--langfuse`（默认开）、`--no-langfuse`、`--odl-hybrid`、`--full`（`neo4j`、`dex`、`searxng`、`langfuse` 等 `full` profile 服务；不含 `collab` 与 `odl-hybrid`）。开发时的 draw.io 编辑器不在 dev 编排里，需要时单独运行 `jgraph/drawio` 并设置 `YUHENG_DOCS_DRAWIO_URL`。

### 2.2 在开发模式下启用在线文档

在线文档默认关闭。在 `.env` 里设 `YUHENG_DOCS_ENABLED=true` 后重启 app 即可使用（不启动协同服务时为独占编辑）。要协同编辑：`make dev-start DEV_ARGS=--docs`，并设置 `YUHENG_COLLAB_URL`（浏览器可访问的 ws 地址）与 `YUHENG_COLLAB_SHARED_SECRET`（至少 16 个字符，app 与 collab 必须一致，否则两边互相拒绝）。全部 `YUHENG_DOCS_*` / `YUHENG_COLLAB_*` 变量在 `.env.example` 中有说明。

### 2.3 后端热重载与断点调试

- **热重载**：安装 Air（`go install github.com/air-verse/air@latest`，并确认 `$(go env GOPATH)/bin` 在 `PATH` 里）。仓库根已带 `.air.toml`，`make dev-app` 检测到 `air` 就用它运行，改完 Go 代码自动重新编译并重启；没装时用普通 `go run`，改完 Ctrl+C 再运行一次即可（秒级）。前端由 Vite 热更新，不需要重启；
- **断点调试**：服务进程自己不读 `.env`，IDE 里启动时要把环境变量交给它，并像 `dev.sh app` 那样把服务地址改成本机端口。VS Code 的 `.vscode/launch.json` 示例：

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/server",
      "envFile": "${workspaceFolder}/.env",
      "env": {
        "DB_HOST": "127.0.0.1",
        "REDIS_ADDR": "127.0.0.1:6379",
        "DOCREADER_ADDR": "127.0.0.1:50051",
        "S3_ENDPOINT": "http://127.0.0.1:9000",
        "NEO4J_URI": "bolt://127.0.0.1:7687"
      }
    }
  ]
}
```

- **docreader 改了代码**：dev 编排里的 docreader 是镜像，改了 Python 代码要重建：`./scripts/build_images.sh -d` 后 `make dev-restart`，或者按 2.4 在本地直接跑；`build_images.sh` 的 `-p` / `-f` / `-d` 分别只构建 app / 前端 / docreader 镜像。

### 2.4 本地单独跑 docreader

```bash
make -C docreader run         # 在仓库根目录执行；按 uv.lock 装依赖后启动 gRPC 服务，监听 DOCREADER_GRPC_PORT（默认 50051）
make -C docreader proto       # 改了 docreader/proto/docreader.proto 后重新生成 pb 代码
```

服务把自己当作 `docreader` 包导入，所以 Python 必须从仓库根目录（`docreader/` 的上一级）启动，和镜像里 `/app/docreader` 的布局一致；在 `docreader/` 里直接 `uv run -m docreader.main` 会报 `No module named 'docreader'`。Makefile 替你切到根目录并用 `--project docreader` 指向 uv 项目。

docreader 的调优参数（PDF 渲染 DPI、扫描件判定、SSRF 白名单、gRPC TLS 等）以 `DOCREADER_*` 环境变量注入，完整清单见 `docker-compose.dev.yml` 的 `docreader.environment` 段。

## 3. Makefile 目标

以下目标定义在根目录 `Makefile`，`make help` 有一份中文帮助。

### 3.1 构建、格式与检查

| 目标 | 作用 |
| --- | --- |
| `build` / `run` | `go build -o Yuheng ./cmd/server`；`run` 先 build 再运行 |
| `build-anydoc` / `anydoc-lib` | 先用 `scripts/build-anydoc-lib.sh` 构建 anydoc 静态库（Rust），再带 `anydoc` 构建标签编译，链接进程内 Office 文档解析器 |
| `build-prod` | 生产构建：CGO_ENABLED=1，`-ldflags "-w -s"` 注入 Version/CommitID/BuildTime/GoVersion（`internal/handler` 包变量）；`GO_BUILD_TAGS=anydoc` 可附加构建标签 |
| `test` | `go test -v ./...` |
| `fmt` / `fmt-check` | `scripts/gofumpt-tree.sh write` / `check`，整棵树 gofumpt，CI 执行同一检查 |
| `lint` | `golangci-lint run` |
| `deps` / `clean` | `go mod download` / 清理二进制 |
| `docs` | `swag init -g ./cmd/server/main.go -d ./,./internal/docs/handler -o ./docs --parseDependency --parseInternal` 生成 Swagger |
| `install-swagger` | 安装 `swag` CLI |
| `download_spatial` | `go run cmd/download/duckdb/duckdb.go` 下载 DuckDB 扩展（表格摘要用） |

### 3.2 Docker 镜像与服务管理

| 目标 | 作用 |
| --- | --- |
| `docker-build-app` / `docker-build-docreader` / `docker-build-frontend` / `docker-build-all` | 本地构建镜像（前端先执行 `scripts/build_frontend_dist.sh`） |
| `build-images` / `build-images-app` / `build-images-docreader` / `build-images-frontend` / `build-images-collab` / `clean-images` | `scripts/build_images.sh` 从源码构建/清理镜像 |
| `docker-run` / `docker-stop` / `docker-restart` | 确保 `.env` 存在后 `docker compose up` / `down` / 重启 |
| `start-all` / `stop-all` / `start-ollama` / `start-docker` / `check-env` / `list-containers` / `pull-images` | `scripts/start_all.sh` 的各个子命令 |
| `show-platform` | 显示 `uname -m` 与 Docker 构建平台 |
| `clean-db` | 删除 compose 里 `postgres-data` / `rustfs_data` / `redis-data` 三个 volume（**清空数据**；项目名向 `docker compose` 查询，不写死 `yuheng_` 前缀；需先 `docker compose down`） |

### 3.3 数据库迁移（详见《数据库与迁移》）

| 目标 | 作用 |
| --- | --- |
| `migrate-up` / `migrate-down` | `scripts/migrate.sh up / down` |
| `migrate-version` | 查看当前迁移版本 |
| `migrate-create name=xxx` | 创建一对新迁移文件 |
| `migrate-force version=N` | 强制设置版本（dirty 恢复） |
| `migrate-goto version=N` | 迁移到指定版本 |

### 3.4 开发模式

| 目标 | 作用 |
| --- | --- |
| `dev-start` / `dev-stop` / `dev-restart` / `dev-logs` / `dev-status` | `scripts/dev.sh` 对应子命令（`DEV_ARGS` 传 profile 参数） |
| `dev-app` / `dev-frontend` | 本地运行后端 / 前端 |

## 4. 测试体系

### 4.1 Go 测试（主模块）

```bash
make test                                   # go test -v ./...
go test ./internal/infrastructure/chunker/...
go test -run TestXxx ./internal/application/service/...
```

**PostgreSQL 是唯一的数据库，测试也一样**：需要数据库的测试调用 `pgtest.New(t)`（`internal/testutil/pgtest`）。每个测试包启动一个 ParadeDB 容器（镜像与 `docker-compose.yml` 相同），用服务端同一套迁移器把 `migrations/versioned` 应用到模板库，每次 `New` 用 `CREATE DATABASE … TEMPLATE` 复制出独立数据库（毫秒级），测试之间看不到彼此的数据、可以并行。另有 `NewURL`、`NewSQL`、`NewEmpty` 变体。仓库里没有 SQLite，也不要添加。

| 环境变量 | 作用 |
| --- | --- |
| `YUHENG_TEST_POSTGRES_URL` | 不启动容器，改用这个 Postgres 超级用户 URL（如 `postgres://postgres:secret@localhost:5432/postgres`） |
| `YUHENG_TEST_POSTGRES_IMAGE` | 换一个镜像（例如镜像仓库里的同版本 ParadeDB） |
| `TESTCONTAINERS_HUB_IMAGE_NAME_PREFIX` | testcontainers 的镜像前缀，让 ParadeDB 与 ryuk 两个镜像都从镜像仓库拉取 |

测试必须在任何时区下通过（会话按 UTC 运行，时间戳按时刻比较）。Redis 相关逻辑用 `miniredis` 替身；部分包依赖 CGO（DuckDB）。`internal/container/e2e_*_test.go` 在真实数据库上装配整个容器，校验启动、认证与在线文档镜像等端到端路径。

### 4.2 docreader 测试（Python）

测试位于 `docreader/tests/`，使用标准库 `unittest`：

```bash
cd docreader
uv sync
uv run python -m unittest discover -s tests -v
```

### 4.3 前端测试

在 `frontend/` 下执行（也可用 `npm --prefix frontend ...`）：

```bash
npm run format:check   # Prettier（printWidth 120）
npm run lint           # ESLint，不允许任何 warning
npm test               # Vitest + happy-dom，断言用 node:assert/strict
npm run type-check     # vue-tsc
npm run check-i18n     # 四个语言包（en-US / zh-CN / ko-KR / ru-RU）键一致
npm run build          # 小内存机器上需 NODE_OPTIONS=--max-old-space-size=4096
```

组件测试用 `@vue/test-utils` 挂载。`scripts/verify_frontend_pr.sh` 按 CI 顺序执行这些检查。

### 4.4 协同服务、CLI 与验收测试

- `collab/`：`npm test`（`tsx --test`）、`npm run type-check`、`npm run build`；
- `cli/` 是独立 Go module，自带 `cli/Makefile`（`make test` / `make test-coverage` / `make lint`）；
- `cli/acceptance/contract/`：envelope JSON 输出形状的 golden 测试与 error.code 注册表一致性；
- `cli/acceptance/e2e/`：对真实 Yuheng 服务的黑盒测试，由 `.github/workflows/cli-e2e.yml` 按需触发（手动，或给 PR 打 `acceptance-e2e` 标签）；
- `scripts/smoke.sh`：按 README 的方式 `docker compose up -d --build` 后走一遍首次使用路径（页面加载、首个账号注册、第二个被拒、登录发 token），CI 中由 `smoke.yml` 执行。

## 5. 代码规范与 CI

### 5.1 Go 代码规范

仓库根 `.golangci.yml`（golangci-lint v2 格式）启用 `lll`（行宽 120）、`govet`、`revive`，格式化器为 `gofmt` + `gofumpt`；`make docs` 生成的 `docs/docs.go` 被排除。仓库里存量 lint 问题较多，CI 只检查新引入的问题（`only-new-issues`）。整树重排格式的提交记录在 `.git-blame-ignore-revs`，不要把格式重排和代码改动混在一个提交里。

提交前：

```bash
make fmt && go vet ./... && golangci-lint run --new-from-rev=origin/main ./... && go test ./...
```

### 5.2 CI 工作流（`.github/workflows/`）

| 文件 | 触发 | 作用 |
| --- | --- | --- |
| `app.yml` | 根模块 Go 代码、`go.mod`、`config/`、`migrations/`、`testdata/`、`docker/Dockerfile.app` 等 | 整树 gofumpt 检查、`go vet`、`go test -count=1 -shuffle=on`（排除 `docreader/`）、`go build ./cmd/server` |
| `go-lint.yml` | 任意 Go 代码或 `.golangci.yml` | golangci-lint，只报告本次改动引入的问题 |
| `frontend.yml` | `frontend/`、`scripts/build_frontend_dist.sh` | Node 24：`format:check` → `lint` → `test` → `type-check` → `build`，另有 `npm audit --omit=dev` |
| `collab.yml` | `collab/`、`packages/docs-schema/` 等 | 协同服务的测试、类型检查与构建 |
| `docreader.yml` | `docreader/`、`testdata/`、相关 Dockerfile | uv 装依赖 → `compileall` → `unittest discover`；再拉起 docreader 跑 `go test ./docreader/client ./docreader/proto` |
| `anydoc.yml` | `third_party/anydoc-go/`、`internal/infrastructure/docparser/` 等 | 构建 anydoc 静态库并 `cargo audit` |
| `smoke.yml` | 除 Markdown、`website-docs/`、`cli/`、`client/`、`packages/`、`mcp-server/`、`helm/` 之外的改动 | 构建镜像并启动默认编排，执行 `scripts/smoke.sh` |
| `cli.yml` / `cli-e2e.yml` | `cli/` / 手动或标签 | CLI 三平台矩阵测试 / CLI 端到端验收 |
| `mcp-server.yml` | `mcp-server/`，标签 `mcp-server-v*` | Python 3.10–3.13 矩阵测试；只有推送标签才会发布到 PyPI |
| `dsh-plugin.yml` | `packages/dsh-yuheng/`，标签 `dsh-yuheng-v*`，定时 | DeepSeek Harness 插件的构建、测试与发布 |
| `license.yml` | 推送 main 等 | 检查 NOTICE 与版权头未被删改，以及依赖许可证可随 MIT 发布（`tools/license_check.sh`） |
| `secrets.yml` | 每次推送与 PR | 扫描整个历史里的凭据（例外见 `.gitleaks.toml`） |
| `docker-image.yml` | 仅 `v*` 标签 | 构建并推送镜像。目前尚未打过发布标签，因此没有已发布的镜像 |

### 5.3 提交流程

项目由一人维护，维护者直接提交到 `main`；外部贡献按 `CONTRIBUTING.md` 提 PR，先开 issue 讨论较大的改动。提交信息用 Conventional Commits 风格（`type(scope): 祈使句摘要`），正文说明为什么改；顺手发现的 bug 在提交信息里点名。每个提交都应通过前端五项检查与 `go test ./...`。新增依赖需要同步 `THIRD_PARTY_NOTICES.md`。

## 6. 调试技巧

### 6.1 日志级别

日志实现在 `internal/logger/logger.go`（logrus）。级别由 `LOG_LEVEL` 控制（`debug` / `info` / `warn` / `error` / `fatal`，未设置或非法时为 **debug**），`LOG_PATH` 控制输出文件。每个请求带 `X-Request-ID`，贯穿 app 与 docreader 日志，排查时先抓 request id。`LLM_DEBUG_LOG=true` 按 request id 记录每次模型调用的完整输入输出。

### 6.2 GIN_MODE 与 Swagger

- `GIN_MODE=release` 时不挂载 Swagger UI（`internal/router/router.go`）；开发时不设或设为 `debug`；
- `make docs` 生成 Swagger 后访问 `http://localhost:8080/swagger/index.html`。

### 6.3 数据库与迁移

- `AUTO_MIGRATE=false` 关闭启动时自动迁移；迁移失败默认终止启动（`MIGRATION_FAIL_FAST`）；`AUTO_RECOVER_DIRTY` 默认关闭；
- `GET /ready` 在数据库、Redis 或迁移状态异常时返回 503 并说明原因，比 `/health` 更适合判断"能不能用"；
- `make migrate-version` 快速确认 schema 版本。

### 6.4 LLM 链路观测（Langfuse）

`dev.sh start` 默认拉起自建 Langfuse（`http://localhost:3000`）。本地 `go run` 的 app 需要导出：

```bash
export LANGFUSE_HOST=http://localhost:3000
export LANGFUSE_PUBLIC_KEY=pk-lf-xxx
export LANGFUSE_SECRET_KEY=sk-lf-xxx
```

文档处理的逐阶段进度另外落在 `knowledge_processing_spans` 表，前端时间线即由此渲染。

### 6.5 后台作业

- 任务队列：系统管理员的运行时面板（`/api/v1/system/admin/runtime/queues`）查看队列深度、失败任务与 worker 心跳；失败到底的任务在 `task_dead_letters`；
- 知识健康：`YUHENG_FINDINGS_ENABLED=false` 关闭检测与复核巡检；日志前缀 `[Findings]`；
- 在线文档镜像：`docs_index_state` 中 `attempts > 0` 或 `last_error` 非空的行就是镜像失败的页面；日志前缀 `[docs]`。启动日志里的 `[docs] running without optional dependencies` 说明有可选依赖缺失，对应功能不可用。

### 6.6 pprof

代码中**未内置** `net/http/pprof` 端点。需要剖析时可临时在 `cmd/server/main.go` 中 `import _ "net/http/pprof"` 并另起 `http.ListenAndServe("localhost:6060", nil)`，或对具体包用 `go test -bench . -cpuprofile`。

### 6.7 分块策略诊断

chunker 提供 `SplitWithDiagnostics()`（`internal/infrastructure/chunker/strategy.go`），返回策略链选择、各 tier 被拒原因与文档画像，配合 `LOG_LEVEL=debug` 可排查分块效果；也可以直接调用 `/api/v1/chunker/preview` 预览分块结果。
