# 平台管理与系统管理员

Yuheng 的权限分两层：**空间内**的四级角色（见[租户、用户与认证授权](01-tenant-auth.md)），以及**平台级**的系统管理员。这一篇讲后者——它管的不是某个知识库，而是整个部署。

先划清界限：

| | 空间 Owner | 系统管理员 |
| --- | --- | --- |
| 作用范围 | 单个工作空间 | 整个部署 |
| 怎么获得 | 注册即成为自己空间的 Owner，或被转让 | 部署的第一个注册用户自动获得；之后由现有系统管理员提升，或用环境变量引导 |
| 管什么 | 空间成员、模型、知识库、数据源、空间审计 | 全局系统设置、任务队列、平台 API Key、跨空间审计、重置用户密码 |
| 是否自动叠加 | — | **不会**：在某空间是 Owner 不代表是系统管理员，反之亦然 |

还有第三个标志 `CanAccessAllTenants`（跨空间访问），它管的是「能不能读写别人的空间数据」，与系统管理员也是分开的：系统管理员默认看不到别人空间里的知识库内容。

## 1. 第一个系统管理员怎么来

**默认情况下，部署的第一个注册用户就是系统管理员。** 注册模式 `auth.registration_mode` 的默认值 `auto` 只在用户表为空时开放注册，此时注册的人被标为系统管理员，之后注册自动关闭（`internal/handler/auth.go` 的 `registrationState`；「表为空才插入」在 `CreateFirstUser` 里用 PostgreSQL advisory lock 保证，两个人同时注册也只会产生一个管理员）。

注册模式改成 `self_serve` 或 `invite_only`、或者第一个用户不是该当管理员的人时，可以用环境变量引导（`cmd/server/bootstrap.go`）：

1. 先用正常流程注册一个账号（或由其他方式创建）；
2. 给 app 服务设 `YUHENG_BOOTSTRAP_SYSTEM_ADMIN_EMAIL=<该账号邮箱>`，重启；
3. 启动时 `bootstrapSystemAdmin()` 检查——**仅当当前部署一个系统管理员都没有时**，把该邮箱对应的用户提升为系统管理员。

几个刻意的设计：

- **不会创建用户**：邮箱还没注册时只打一条 WARN，下次重启再试。账号创建涉及密码哈希、空间分配、审计，不适合在启动钩子里走捷径；
- **幂等且会自动失效**：一旦存在系统管理员，这个变量就不再授权——避免管理员在界面上刚撤销的权限被下次重启悄悄恢复。所以它可以长期留在部署清单里；
- **失败不阻断启动**：整个 bootstrap 是 best-effort，配错了变量也能把服务拉起来再改。

之后新增/移除管理员在「系统设置」页里操作即可（对应 `POST /system/admin/promote` / `revoke`，`GET /system/admin/list` 列出现有管理员）。撤销有两道保险：**不能撤销自己**，也**不能撤销最后一个系统管理员**——否则平台会永久失去系统级管理能力。对已经不是管理员的用户重复撤销返回 200（幂等），但审计记录里 `changed=false`，便于事后区分真实撤销与空操作。

## 2. 控制台能做什么

界面入口在「设置」侧栏的「系统管理」分组，仅系统管理员可见（前端白名单见 `frontend/src/config/settingsAccess.ts` 的 `SYSTEM_ADMIN_SETTINGS_SECTIONS`），四个分区：

| 分区 | 作用 | 接口 |
| --- | --- | --- |
| 系统设置 | 全局运行时开关（注册模式、空间策略、上传大小、并发、SSRF 白名单、集中管理基础设施等），见 §3；也在这里提升/撤销系统管理员 | `GET/PUT/DELETE /system/admin/settings[/:key]` |
| 任务队列 | 查看 asynq 各队列实时积压、逐个任务的重试/归档/删除、批量清空归档任务；无 Redis 时返回 `available=false` | `/system/admin/runtime/queues*` |
| 平台 API Key | 面向控制面自动化的 platform 作用域 Key，能力包括 `system_tenants_read/manage`、`system_settings_read/manage`、`system_runtime_read/manage`、`system_audit_read` | `/system/admin/api-keys` |
| 系统审计日志 | `tenant_id = 0` 的平台级事件（改设置、提升/撤销管理员、队列操作等）。空间级审计接口按 tenant 过滤，看不到这些行 | `GET /system/admin/audit-log` |

另外几个不在上表、但同属系统管理员的能力：

- **创建用户**（`POST /system/admin/users/create`）：直接建一个本地账号，空间分配遵循 `auth.default_tenant_mode`；身份已存在时返回已有用户；
- **重置用户密码**（`POST /system/admin/users/reset-password`）：替换目标用户的本地密码并吊销其全部会话。**不能给自己重置**——自助改密码仍要求提供旧密码；
- **平台级解析引擎默认配置**（`GET/PUT /system/admin/parser-engine-config`，界面在「设置 → 解析引擎 → 平台默认」）：介于部署环境变量与各空间自己的覆盖之间；
- **批量套用默认存储配额**（`POST /system/admin/tenants/apply-default-storage-quota`）：把当前的默认配额写到所有已存在的空间上。之所以挂在 `/tenants` 而不是 `/settings` 下，是因为它改的是空间数据而不是设置行。

## 3. 运行时可改的系统设置

`internal/application/service/system_setting.go` 维护一张注册表，表内的键可以在控制台里改，**数据库值盖过环境变量**；除标明需要重启的，改完立即生效：

| 键 | 类型 | 默认（对应环境变量） | 生效时机 |
| --- | --- | --- | --- |
| `file.max_size_mb` | int | `50`（`MAX_FILE_SIZE_MB`） | 立即；默认部署下最大可调到 512（docreader gRPC 硬顶 `DOCREADER_GRPC_MAX_FILE_SIZE_MB`） |
| `file.video_max_size_mb` | int | `2048`（`MAX_VIDEO_FILE_SIZE_MB`） | 立即；超过前端 nginx 启动时的请求体上限需改环境变量并重启前端 |
| `auth.registration_mode` | `auto` / `self_serve` / `invite_only` | `auto`（只在还没有用户时开放，首个注册者成为系统管理员） | 立即 |
| `auth.default_tenant_mode` | `create_personal` / `tenantless` | `create_personal`（`YUHENG_AUTH_DEFAULT_TENANT_MODE`） | 只影响之后注册的新用户 |
| `tenant.self_service_creation_enabled` | bool | `true` | 立即 |
| `tenant.max_owned_per_user` | int | `10`（0 = 用内置默认，负数 = 关闭限额） | 每次建空间时读取 |
| `tenant.default_storage_quota_gb` | int | `10` | **仅新建空间时读取**，不回写已有空间 |
| `tenant.auto_accept_invitation` | bool | `false` | 立即；开启后邀请已注册用户时直接加入空间，不再等对方接受 |
| `ssrf.whitelist` | 字符串列表 | 空（`SSRF_WHITELIST`） | 立即（`SSRF_WHITELIST_EXTRA` 仍只由部署方维护，不在此覆盖） |
| `asynq.core/postprocess/enrichment/maintenance/shared/wiki_concurrency` | int | 见[异步任务系统](../02-architecture/05-async-tasks.md) | **需重启** |
| `model.max_concurrency` | int | `32`（`YUHENG_MODEL_MAX_CONCURRENCY`） | 立即；只限制后台任务，不影响交互式对话 |
| `governance.centralized_infra` | bool | `false` | 立即；开启后模型、网络搜索、向量存储、存储后端、解析引擎、Ollama 的写操作只允许系统管理员，空间 Owner/Admin 保留只读（`PlatformManaged` 路由守卫） |

::: warning 配置来源不只有环境变量
上表这些键一旦在控制台里改过，数据库里就有了一行记录，**之后改环境变量不再有效果**。排查「明明改了 env 却没生效」时先看这里；把设置项重置（`DELETE /system/admin/settings/:key`）会删掉 DB 行，重新回落到环境变量或内置默认值。
:::

## 4. 集中管控基础设施 {#centralized-infra}

面向内部部署：平台运维统一配置模型、解析引擎、向量存储、存储后端、网络搜索等基础设施，其他注册用户只管使用，不接触凭据。它与空间 RBAC、共享空间正交——RBAC 管「一个空间里谁能做什么」，共享空间管「知识库怎么跨空间共享」，这里管「基础设施归谁配」。

### 为什么空间角色解决不了

每个自助注册的用户都会得到一个自己的个人空间，并成为该空间的 Owner；角色是分级的，Owner 天然满足 Admin。所以把某个设置页的门槛从 Viewer 提到 Admin 甚至 Owner，对自助注册用户没有约束力。要表达「只有平台运维能配」，需要一个与空间角色正交的维度，这就是 `governance.centralized_infra`。

### 开启

在「设置 → 系统管理 → 系统设置」的「访问」一栏打开「集中管控基础设施」，立即生效、无需重启。也可以用环境变量 `YUHENG_GOVERNANCE_CENTRALIZED_INFRA=true` 作为部署期默认值（数据库里的值优先，见 §3 的提示）。

前置条件：

1. **先有系统管理员**，见 §1；
2. **配好 `SYSTEM_AES_KEY`**：没有它时，模型、存储后端、数据源、API Key 等凭据以明文落库（加密只在配置了这个密钥时进行）。资源共享出去意味着更多人依赖这份数据，这是前置条件而不是可选项。

开关关闭时行为与未开启前完全一致，可以先验证再切。

### 开启后的变化

- 下列设置分区对非系统管理员隐藏（前端表 `PLATFORM_MANAGED_SETTINGS_SECTIONS`）：模型管理、Ollama、向量数据库引擎、解析引擎、存储引擎、网络搜索；
- 对应的写接口走 `PlatformManaged` 路由守卫，只允许系统管理员；空间 Owner / Admin 保留只读；
- **入口隐藏不影响使用**：建知识库时的选择器走各自的 Viewer+ 只读接口，平台资源照常出现在下拉框里，只是凭据与端点被响应 DTO 抹掉；
- 空间级的事务不受影响：空间信息、成员、消息管理、API Key 仍由空间 Owner / Admin 管理。设为空间默认存储后端也仍是空间 Admin 的权限。

### 平台共享资源

集中管控回答「谁能配」，共享回答「配好的东西谁能用」：普通用户失去写权限之后，需要有平台提供的实例可选。共享与取消共享只有系统管理员能操作（`authorizePlatformSharingChange`），被共享的行标记为 `is_builtin`，对所有空间可见：

| 资源 | 怎么共享 | 取消共享的护栏 |
| --- | --- | --- |
| 模型 | 模型卡片菜单 → 设为平台共享 | 仍被任意空间的知识库绑定时拒绝 |
| 向量存储 | 卡片菜单 → 设为平台共享 | 其他空间仍有知识库绑定时拒绝 |
| 存储后端 | 卡片菜单 → 设为平台共享（`PUT /storage-backends/:id/sharing`） | 其他空间仍有默认存储、知识库等绑定时拒绝 |
| 网络搜索 | 卡片菜单 → 设为平台共享（`PUT /web-search-providers/:id/sharing`） | 无（按次选用，失效时退化为联网搜索不可用） |
| 解析引擎 | 「平台默认」档位，见下 | 不适用（是配置块，不是实例） |

共享资源对非系统管理员只读：凭据从不返回，端点、Base URL 与额外配置也一并抹掉，只留名称、类型等够选择器渲染的信息。

护栏之所以必要：`knowledge_bases` 上的 `embedding_model_id`、`vector_store_id`、`storage_backend_id` 都是没有外键约束的字符串列。取消共享或删除一个还被别的空间引用的资源，那个空间的检索会静默失效——不报错，只是查不出东西。所以这些操作放行前会做跨空间引用扫描。存储后端与网络搜索要先取消共享再删除：删除路径上的引用检查按空间计，直接删会留下其他空间的悬空引用。

### 解析引擎的三层配置

解析引擎不是实例列表而是一个配置块，所以走继承而不是共享：

```
部署环境变量  <  平台默认（系统管理员）  <  空间覆盖
```

- 部署环境变量在 docreader 内部生效，每个引擎读自己的变量；
- 平台默认在「设置 → 解析引擎 → 平台默认」（仅系统管理员，接口 `GET/PUT /system/admin/parser-engine-config`）；
- 空间覆盖在「设置 → 解析引擎 → 本空间」，集中管控开启后对非系统管理员隐藏。

合并是**逐字段**的（`internal/types/parser_engine_platform.go`）：只覆盖了 MinerU 端点的空间，仍然继承平台的其余引擎设置。布尔开关用指针类型正是为此——空间显式关掉某个选项会覆盖平台默认的开启，空间从未提及的字段才落到平台默认。

### 运维注意

- **Embedding 模型换不得**：已被知识库使用的 embedding 模型一换，已有向量全部失效，必须重建索引。
- **额度没有护栏**：模型共享出去后所有空间共用同一份 API 额度；`model.max_concurrency` 限制的是并发，不是配额。
- **收紧注册**：内网可达但不隔离时，把 `auth.registration_mode` 设为 `invite_only`，避免陌生人注册后使用平台模型。
- **YAML 与界面的所有权**：系统管理员在界面上保存过某个由 `config/builtin_models.yaml` 下发的内置模型后，该行的 `managed_by` 被清空，YAML 之后不再覆盖它；反过来，界面拒绝删除仍由 YAML 托管的行（删了下次启动会被写回）。
- **改入口规则时前后端一起改**：前端表 `PLATFORM_MANAGED_SETTINGS_SECTIONS` 与后端 `PlatformManaged` 路由不一致，会出现「能打开但存不进去」或「界面没有但接口还能调」。

## 相关

- 空间内的四级角色与 API Key：[租户、用户与认证授权](01-tenant-auth.md)
- 队列拓扑与 worker pool：[异步任务系统](../02-architecture/05-async-tasks.md)
- 审计日志与追踪：[可观测性与审计](16-observability.md)
- 接口清单：[API 参考：系统与平台管理](../04-api/02-api-system.md)

## 实现参考

| 路径 | 内容 |
|---|---|
| `cmd/server/bootstrap.go` | `YUHENG_BOOTSTRAP_SYSTEM_ADMIN_EMAIL` 引导 |
| `internal/handler/auth.go`、`internal/application/repository/user.go`（`CreateFirstUser`） | 首个注册者成为系统管理员 |
| `internal/handler/system.go` | 提升/撤销管理员、创建用户、重置密码、平台 API Key |
| `internal/application/service/system_setting.go`、`internal/types/system_setting.go` | 运行时系统设置注册表 |
| `internal/router/routes_auth_tenant.go` | `/system/admin/*` 路由 |
| `internal/router/rbac.go`、`internal/middleware/rbac.go`（`RequirePlatformManaged`） | `PlatformManaged` 守卫 |
| `internal/application/service/platform_shared_infra.go` | 共享资源的写守卫 |
| `internal/types/parser_engine_platform.go` | 解析引擎配置的分层合并 |
| `frontend/src/config/settingsAccess.ts`（`SYSTEM_ADMIN_SETTINGS_SECTIONS`、`PLATFORM_MANAGED_SETTINGS_SECTIONS`）、`frontend/src/views/system/SystemSettings.vue` | 控制台入口与系统设置页 |
