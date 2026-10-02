# 存储后端（Storage Backends）

原始文件、解析出的图片、导出产物都要落在某个存储上。Yuheng 只支持两种存储类型：**本机目录（`local`）** 和 **S3 兼容对象存储（`s3`）**，没有为各家云单独集成 SDK——阿里云 OSS、腾讯云 COS、火山引擎 TOS、华为云 OBS 都通过它们的 S3 兼容端点接入。

存储只有一个配置来源：`storage_backends` 表。部署本身的存储（环境变量配置的那一套）是表里一条固定记录 `env`；空间可以再**注册多个存储实例**，选一个作默认，知识库和文档空间各自绑定到一个实例。

一条规则贯穿全篇：**绑定只决定新文件写到哪里，从不决定旧文件从哪里读**。每个文件写入时都登记一条资源记录（`resources`），记下它所在的后端和后端内的位置；之后所有读取都按这条记录走。所以换绑定、改空间默认，都不会让已有文件失联。

典型用途：

- 不同团队/项目的资料落在不同的桶，便于分账与权限隔离；
- 合规要求某类文档必须存在特定地域的桶里；
- 从自带的 RustFS 迁到外部对象存储时，新库先用新后端，老库保持不动。

## 默认配置：自带的 RustFS

docker compose 默认就用自带的 RustFS（S3 兼容对象存储），不需要任何配置：

| 项 | 默认值 |
| --- | --- |
| `STORAGE_TYPE` | `s3` |
| 端点 | `http://rustfs:9000`（`S3_ENDPOINT`） |
| 区域 | `us-east-1`（`S3_REGION`） |
| 桶 | `yuheng`（`S3_BUCKET_NAME`，首次使用时自动创建） |
| 路径前缀 | `yuheng/`（`S3_PATH_PREFIX`） |
| 凭证 | `S3_ACCESS_KEY` / `S3_SECRET_KEY`，默认取 `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`，两者默认都是 `rustfsadmin` |

`rustfs` 服务不属于任何 profile，始终随 compose 启动（即使 `STORAGE_TYPE` 指向别处，也保持同样的启动顺序）。镜像按 digest 固定；API 端口 9000 与控制台端口 9001 默认只绑定 `127.0.0.1`（`RUSTFS_BIND`、`RUSTFS_PORT`、`RUSTFS_CONSOLE_PORT`）。默认凭证只适合首次试用：把 `RUSTFS_BIND` 改成非回环地址时，必须同时设置自己的 `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`，compose 无法替你检查。应用容器默认把 `rustfs` 主机名加进 SSRF 白名单（`SSRF_WHITELIST_EXTRA=searxng,rustfs`）。

其他取值：

- `STORAGE_TYPE=local`：文件写到 `LOCAL_STORAGE_BASE_DIR`（默认 `/data/files`），只适合单机；
- 外部 S3 兼容存储：设置 `S3_ENDPOINT`、`S3_REGION`、`S3_ACCESS_KEY`、`S3_SECRET_KEY`、`S3_BUCKET_NAME`、`S3_PATH_PREFIX`、`S3_USE_SSL`、`S3_ADDRESSING_STYLE`。Access Key 与 Secret Key 必须同时填写或同时留空，同时留空时使用 AWS SDK 默认凭证链（IAM Role、IRSA 等）。`S3_ADDRESSING_STYLE` 取 `auto`（默认，AWS 端点用虚拟主机式、其他用路径式）/ `path` / `virtual`；OSS、COS、TOS、OBS 必须设为 `virtual`，RustFS、MinIO 用 `path` 或 `auto`；
- `STORAGE_ALLOW_LIST`：逗号分隔的允许类型白名单（如 `local,s3`），留空允许全部；
- 要让外部客户端直接加载预签名链接时，端点必须对客户端可达（不能是 `rustfs:9000`），见[图片与文件的对外访问](21-file-access.md)。

### Helm 部署

Helm chart（`helm/`）用 `storage.type` 选部署存储，取值与 compose 对应：

| `storage.type` | 等价于 compose 的 | 说明 |
| --- | --- | --- |
| `rustfs`（默认） | 默认配置（自带 RustFS） | chart 自己跑 RustFS（同一个按 digest 固定的镜像，单副本 + PVC，Service 名 `rustfs`）；凭证 `secrets.storageAccessKey` / `storageSecretKey` 必填，RustFS 与 app 共用这一对 |
| `s3` | 设置 `S3_*` 指向外部服务 | `storage.s3.endpoint` / `region` / `bucket` / `pathPrefix` / `useSSL` / `addressingStyle`；密钥同填或同空（同空走 AWS 默认凭证链，如 IRSA） |
| `local` | `STORAGE_TYPE=local` | 挂进 app 的 ReadWriteOnce 卷，只支持一个 app 副本（`app.replicaCount` > 1 时 chart 拒绝渲染）；docreader 只读挂同一个卷按路径直读大视频 |

与 compose 相同，桶由 app 首次使用时自动创建；chart 会把存储端点主机加进 `SSRF_WHITELIST_EXTRA`。`STORAGE_ALLOW_LIST` 由 `storage.allowList` 设置，留空时只放行 `s3`（`storage.type=local` 时再加 `local`）——其他类型下没有卷挂在 `LOCAL_STORAGE_BASE_DIR`，空间注册的本机目录实例会把文件写进 Pod 自己的文件系统、随 Pod 一起丢失。参数表与升级说明见 `helm/README.md` 的「File storage」与「Upgrading」。

### 部署存储：`env` 记录

环境变量配置的这套存储在 `storage_backends` 里是**一条**记录：`id = env`、`source = env`、不属于任何空间（`tenant_id` 为空）、共享给所有空间（`is_builtin`），界面上名为「Deployment storage」。它只读：不能通过接口修改、删除或取消共享。

- **每次启动同步**：服务启动时按当前环境变量重写这条记录的类型与位置（端点、区域、桶、路径前缀、寻址方式等）；
- **不存密钥**：`S3_ACCESS_KEY` / `S3_SECRET_KEY` 从不写进数据库，每次建驱动时从环境变量读取，所以轮换密钥后重启即可生效；
- **拒绝错误配置启动**：`STORAGE_TYPE` 不受支持、被 `STORAGE_ALLOW_LIST` 排除、S3 配置不完整（缺区域或桶、两把密钥只填一把），服务都会拒绝启动并说明原因；
- **拒绝把文件“搬丢”**：如果环境变量指向了与记录不同的位置（改了 `STORAGE_TYPE`、端点、区域、桶或路径前缀），而这条记录上还存着文件，服务拒绝启动——这些文件都按记录里的位置解析，静默改写会让它们全部读不到。要换部署存储，先迁移文件；记录上没有文件时，改位置就只是改配置。

新建的空间默认就用它（`tenants.default_storage_backend_id = env`），不再为每个空间复制一份。

## 在界面上怎么配

入口在「设置 → 存储引擎」（「数据与扩展」分组）：

1. 新建后端，选 provider（`local` 或 `s3`）。选 `s3` 时可以从预设填充端点、区域与寻址方式：RustFS、MinIO、AWS S3、阿里云 OSS、腾讯云 COS、火山引擎 TOS、华为云 OBS；
2. **保存前点「测试」**：连通性测试会真实读写一次，配错的桶或过期的密钥能立刻发现，而不是等到上传文档时才报错；
3. 需要的话把它设为空间默认（`PUT /storage-backends/:id/default`，写入 `tenants.default_storage_backend_id`）。空间能看到的任何启用中的实例都可以设为默认：自己的、平台共享的、部署存储 `env`。新建知识库和文档空间不指定实例时用它；聊天图片、会话附件、临时文档这些不属于任何知识库的文件也写到它；
4. 单个知识库想用别的实例，在知识库设置「存储与数据」分组的「存储引擎」页里选——对应 `knowledge_bases.storage_backend_id`。文档空间的绑定是 `docs_spaces.storage_backend_id`（`PUT /docs/spaces/:id/knowledge-base` 的 `storage_backend_id`，传空字符串表示改回空间默认）。

新建、修改、删除、测试后端需要空间 Admin；开启集中管理基础设施后只有系统管理员可以操作，系统管理员还可以把一个后端共享给所有空间（`PUT /storage-backends/:id/sharing`，`is_builtin`）。设为空间默认始终是空间 Admin 的权限。

## 接口

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET | `/storage-backends/types` | Viewer+，返回 `STORAGE_ALLOW_LIST` 允许的 provider 及其字段定义 |
| GET | `/storage-backends`、`/storage-backends/:id` | Viewer+ |
| POST | `/storage-backends` | PlatformManaged（Admin+，集中管理时仅系统管理员） |
| PUT / DELETE | `/storage-backends/:id` | PlatformManaged |
| POST | `/storage-backends/test` | PlatformManaged，用未保存的参数试连 |
| POST | `/storage-backends/:id/test` | PlatformManaged，测已保存的实例 |
| PUT | `/storage-backends/:id/sharing` | PlatformManaged，共享给全平台（服务层限定系统管理员） |
| PUT | `/storage-backends/:id/default` | Admin+，设为空间默认 |

API Key 需要 `manage_storage_backends` 能力或 full-access。

## 数据模型与几个约束

`storage_backends` 表（软删除）关键字段：

| 字段 | 说明 |
| --- | --- |
| `tenant_id` | 所属空间；`env` 记录为空（约束保证只有 `source = env` 的那条没有空间） |
| `name` | 空间内唯一（软删除下的部分唯一索引） |
| `provider` | `local` / `s3` |
| `config` | JSON：`endpoint`、`region`、`access_key_id`、`secret_access_key`（加密存储；`env` 记录不存）、`bucket_name`、`path_prefix`、`use_ssl`、`addressing_style` |
| `source` | `user`（空间注册）/ `env`（部署存储，全表唯一一条） |
| `status` | `active` / `disabled`；停用的实例不再接收新文件 |
| `is_builtin` | 平台共享给所有空间 |

绑定都是**必填**的：`tenants.default_storage_backend_id`、`knowledge_bases.storage_backend_id`、`docs_spaces.storage_backend_id` 都不能为空。文件的位置记在资源记录上：`resources.storage_backend_id`（必填）加后端内的原生位置（`local://<相对路径>` 或 `s3://<桶>/<键>`），对外只暴露 `resource://<handle>`。

**有文件的知识库也可以换实例**：换绑定后，已有文件仍在原实例上、照常可读，此后新增的文件写到新实例。知识库设置页在这种情况下会提示这一点，而不是禁止选择。文档空间同理。

**复制与移动**：知识库之间复制原始文件是服务端复制，要求两边绑定同一个实例；绑定不同实例的知识库之间复制会被拒绝。删除知识库或文件时，每个文件都从它实际所在的实例上清理，与知识库当前的绑定无关。

**停用、删除与取消共享**：只要有东西还在用，就会被拒绝——任何空间里把它设为默认、绑定它的知识库或文档空间、存在它上面的有效文件，跨空间统一计数：

| 操作 | 拒绝条件 |
| --- | --- |
| 停用 | 上述任何一项 |
| 删除 | 上述任何一项；另外共享中的实例须先取消共享 |
| 取消共享 | 所属空间之外还有上述任何一项（所属空间自己的使用不受影响） |

## 与向量存储的区别

两者容易混：

| | 存储后端（Storage Backend） | 向量存储（Vector Store） |
| --- | --- | --- |
| 存什么 | 原始文件、图片、导出产物 | 向量与检索索引 |
| 配在哪 | 「设置 → 存储引擎」 | 「设置 → 向量数据库引擎」 |
| 知识库字段 | `storage_backend_id` | `vector_store_id` |
| 相关章节 | 本篇 | [检索引擎与向量存储](05-retrieval-engines.md) |

## 实现参考

| 路径 | 内容 |
|---|---|
| `internal/types/storagebackend.go` | 数据模型、配置加解密、`EnvStorageBackend`（从环境变量构造部署存储） |
| `internal/application/service/storagebackend.go` | 后端 CRUD、连通性测试、默认后端、跨空间绑定计数 |
| `internal/application/service/storage_store.go` | `FileStore`：按资源记录读、按绑定写 |
| `internal/handler/storagebackend.go` | HTTP 接口 |
| `internal/router/routes_infra.go` | `RegisterStorageBackendRoutes` |
| `internal/container/storage_sync.go` | 启动时同步 `env` 记录 |
| `internal/storageallowlist/allowlist.go` | `STORAGE_ALLOW_LIST` |
| `migrations/versioned/000068_storage_backends.up.sql`、`000134`–`000137` | 表结构；`env` 记录、必填绑定、按后端唯一的资源位置 |
| `frontend/src/views/settings/StorageBackendSettings.vue`、`s3Presets.ts` | 「设置 → 存储引擎」与 S3 预设 |
| `frontend/src/views/knowledge/settings/KBStorageSettings.vue` | 知识库的存储选择 |
| `docker-compose.yml` 的 `rustfs` 服务、`.env.example` 的 B3/B4 节 | 自带 RustFS 与存储环境变量 |
| `helm/values.yaml` 的 `storage` 节、`helm/templates/rustfs.yaml` | Helm 部署的存储配置与自带 RustFS |
