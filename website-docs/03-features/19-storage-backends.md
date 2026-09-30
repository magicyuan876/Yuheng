# 存储后端（Storage Backends）

原始文件、解析出的图片、导出产物都要落在某个存储上。Yuheng 只支持两种存储类型：**本机目录（`local`）** 和 **S3 兼容对象存储（`s3`）**，没有为各家云单独集成 SDK——阿里云 OSS、腾讯云 COS、火山引擎 TOS、华为云 OBS 都通过它们的 S3 兼容端点接入。在此之上可以**注册多个存储实例**（migration `000068` 起），空间选一个作默认，单个知识库还能绑定到指定实例。

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

环境变量配置的这套存储在每个空间里以一条只读的「System S3」（或「System LOCAL」）记录出现（`source = env`），每次启动按当前环境变量刷新，所以轮换密钥后重启即可生效。空间还没有默认后端时，它就是默认；未绑定后端的知识库也会绑到它。

## 在界面上怎么配

入口在「设置 → 存储引擎」（「数据与扩展」分组）：

1. 新建后端，选 provider（`local` 或 `s3`）。选 `s3` 时可以从预设填充端点、区域与寻址方式：RustFS、MinIO、AWS S3、阿里云 OSS、腾讯云 COS、火山引擎 TOS、华为云 OBS；
2. **保存前点「测试」**：连通性测试会真实读写一次，配错的桶或过期的密钥能立刻发现，而不是等到上传文档时才报错；
3. 需要的话把它设为空间默认（`PUT /storage-backends/:id/default`，同时写回 `tenants.default_storage_backend_id`）。新建知识库不指定实例时就用这个默认值；
4. 单个知识库想用别的实例，在知识库设置「存储与数据」分组的「存储引擎」页里选——对应 `knowledge_bases.storage_backend_id`。

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

`storage_backends` 表（`tenant_id` 隔离，软删除）关键字段：

| 字段 | 说明 |
| --- | --- |
| `name` | 空间内唯一（软删除下的部分唯一索引） |
| `provider` | `local` / `s3` |
| `config` | JSON：`endpoint`、`region`、`access_key_id`、`secret_access_key`（加密存储）、`bucket_name`、`path_prefix`、`use_ssl`、`addressing_style` |
| `source` | `user`（界面注册或由旧配置迁移）/ `env`（环境变量快照） |
| `status` | `active` / `disabled` |
| `legacy_alias` | 见下 |
| `is_builtin` | 平台共享给所有空间 |

**`legacy_alias` 是为平滑升级准备的**：多实例模型之前，存储配置是空间级的单份 JSON 或环境变量；启动时它们被折算成别名记录（`StorageBackendFromLegacy` / `StorageBackendFromEnvironment`），让老知识库的文件路径继续可解析，而不必做数据搬迁。同一空间同一 provider 只允许一条别名（部分唯一索引保证），因此它不会和你手工注册的实例混淆。

**库里一有文件就不能再换**：知识库的存储选择在**空库时可改**，一旦有了文件，界面上的选择框就被禁用并提示需要迁移（`KBStorageSettings.vue` 按 `hasFiles` 判断）。原因是已入库文件的路径是按当时的后端生成的，直接改绑定会让旧文件失联。确实要换的话，新建知识库再迁移内容。

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
| `internal/types/storagebackend.go` | 数据模型、配置加解密、`StorageBackendFromLegacy` / `StorageBackendFromEnvironment` |
| `internal/application/service/storagebackend.go` | 后端 CRUD、连通性测试、默认后端 |
| `internal/handler/storagebackend.go` | HTTP 接口 |
| `internal/router/routes_infra.go` | `RegisterStorageBackendRoutes` |
| `internal/container/container.go` | 启动时把旧配置与环境变量折算成别名记录 |
| `internal/storageallowlist/allowlist.go` | `STORAGE_ALLOW_LIST` |
| `migrations/versioned/000068_storage_backends.up.sql` | 表结构 |
| `frontend/src/views/settings/StorageBackendSettings.vue`、`s3Presets.ts` | 「设置 → 存储引擎」与 S3 预设 |
| `frontend/src/views/knowledge/settings/KBStorageSettings.vue` | 知识库的存储选择 |
| `docker-compose.yml` 的 `rustfs` 服务、`.env.example` 的 B3/B4 节 | 自带 RustFS 与存储环境变量 |
