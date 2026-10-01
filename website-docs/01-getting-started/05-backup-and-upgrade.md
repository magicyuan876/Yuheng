# 备份、升级与多副本

本文回答三件运维上绕不开的事：数据到底存在哪里、怎么一致地备份和恢复、升级时迁移失败了怎么办。示例命令都以标准 `docker-compose.yml` 的服务名为准，在仓库根目录执行。

> **升级前先备份。** 数据库迁移在实践中只能向前：`.down.sql` 文件存在，但迁移里有些步骤会不可逆地丢数据（见[已发布的破坏性迁移](#已发布的破坏性迁移)），回滚版本的可靠办法是「恢复备份 + 换回旧版本镜像」。

## 状态存在哪里

| 状态 | 位置 | 说明 |
| --- | --- | --- |
| 业务数据库 | 卷 `postgres-data`（服务 `postgres`） | 租户、用户、知识库、文档元数据、切片与向量、会话消息、在线文档正文等**几乎所有结构化数据**；迁移版本记录表 `schema_migrations` 也在这里 |
| 上传的原始文件和解析出的图片 | 默认（`STORAGE_TYPE=s3`）在卷 `rustfs_data`（服务 `rustfs`）；`STORAGE_TYPE=local` 时在卷 `data-files`（挂在 `/data/files`）；接外部 S3 时在你的对象存储里 | 数据库只存指向它们的路径，**只备份数据库不够** |
| 在线文档的协同状态 | 同样在 `postgres`（`docs_pages.ydoc`） | 协同服务 `collab` 自己不落盘、不持有数据，无需单独备份 |
| Redis | 卷 `redis-data`（服务 `redis`，AOF 持久化） | 异步任务队列和缓存。丢失后排队中和处理中的任务会消失，卡在「处理中」的文档由巡检回收，需要时重新解析；不是必须备份的数据 |
| 知识健康 | 同样在 `postgres`（`knowledge_findings` 等表，以及文档的负责人与复核记录） | 问题记录可以重新检测出来，但「忽略」的决定、手动指派、负责人与「确认仍然有效」的记录只存在这里 |
| 知识图谱 | 卷 `neo4j-data`（服务 `neo4j`，仅启用 `neo4j` profile 时） | 可以由文档重新抽取，但耗时耗模型费用 |
| 临时目录 | 卷 `docreader-tmp` | 解析中间产物，不用备份 |
| 其他可选组件 | `searxng_config`、`langfuse_*` 等 | 各自独立，Langfuse 的数据库 `langfuse` 与业务库在同一个 postgres 里，`pg_dump` 时按需一起导出 |

`.env` 里的 `SYSTEM_AES_KEY` 和 `JWT_SECRET` 也是状态的一部分：数据库里加密存放的 API Key、模型密钥、数据源凭据**离开 `SYSTEM_AES_KEY` 就无法解密**，备份时把 `.env` 一起收好（放在与数据备份不同的地方）。

## 一致地备份

数据库和对象存储是两套存储，不在同一个事务里。要得到「数据库里引用的文件都在」的一致备份，最稳妥的办法是**短暂停写再备份**（冷备份）；不能停机时用热备份，代价是两者可能差几秒。

### 冷备份（推荐）

```bash
mkdir -p backup && set -a && . ./.env && set +a
STAMP=$(date +%F-%H%M)

# 1. 停掉会写数据的服务；postgres 和 rustfs 保持运行
docker compose stop frontend app collab

# 2. 数据库：自定义格式（-Fc），可并行、可选择性恢复。-T 关闭伪终端，否则会损坏二进制输出
docker compose exec -T postgres pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "backup/yuheng-$STAMP.dump"

# 3. 对象存储（默认的 RustFS 卷）。卷名带 compose 项目前缀，先用 docker volume ls | grep rustfs 确认
docker compose stop rustfs
docker run --rm -v "$(docker volume ls -q | grep 'rustfs_data$' | head -1)":/data:ro -v "$PWD/backup":/backup \
  busybox tar czf "/backup/rustfs-$STAMP.tgz" -C /data .
docker compose start rustfs

# 4. 起服务
docker compose start app collab frontend
```

- 没有启用 `docs` profile 时，第 1、4 步去掉 `collab`（没有这个容器时 `start` 会报错）。启用了 `full` profile 的，同样先停 `mcp`。
- `STORAGE_TYPE=local`：第 3 步换成 `data-files` 卷（`grep 'data-files$'`）。
- 外部 S3（云厂商）：用厂商的版本控制 / 跨区域复制，或 `rclone sync` 到另一个桶；备份时间点要与数据库转储相近。
- 用了知识图谱：同样方式打包 `neo4j-data` 卷（先 `docker compose stop neo4j`）。
- 备份文件要拷到**这台机器之外**，并定期演练恢复。

### 热备份（不停机）

`pg_dump` 本身给出的是一致的数据库快照，可以直接在服务运行时执行第 2 步。之后再拷贝对象存储：这样备份里的文件是数据库快照之后的状态，**只会多出文件，不会缺文件**（快照时刻引用的文件之后被删除的情况除外）。恢复后多出来的文件只是占用空间。

## 恢复

数据库和对象存储要**一起恢复到同一个备份时间点**，并使用与备份时相同的 postgres 镜像版本（`paradedb/paradedb:v0.22.2-pg17`），否则 `vector` / `pg_search` 扩展的版本可能对不上。

```bash
set -a && . ./.env && set +a
docker compose stop frontend app collab

# 1. 重建空库并恢复
docker compose exec -T postgres dropdb -U "$DB_USER" --if-exists "$DB_NAME"
docker compose exec -T postgres createdb -U "$DB_USER" "$DB_NAME"
docker compose exec -T postgres pg_restore -U "$DB_USER" -d "$DB_NAME" --no-owner < backup/yuheng-<STAMP>.dump

# 2. 对象存储：停 rustfs，清空卷后解包
docker compose stop rustfs
docker run --rm -v "$(docker volume ls -q | grep 'rustfs_data$' | head -1)":/data -v "$PWD/backup":/backup \
  busybox sh -c 'rm -rf /data/* /data/.[!.]* 2>/dev/null; tar xzf /backup/rustfs-<STAMP>.tgz -C /data'
docker compose start rustfs

# 3. 起服务；如果备份来自更旧的版本，启动时会自动向前迁移
docker compose start app collab frontend
```

没有启用 `docs` profile 时，同样去掉命令里的 `collab`。恢复后用 `curl localhost:8080/ready` 确认返回 200（见下文）。

## 升级流程

1. **读 `CHANGELOG.md`**，特别留意破坏性变更和「已发布的破坏性迁移」表里是否有新条目。
2. **备份**（上一节）。这一步不可省：迁移失败时唯一保证能回到原样的办法就是它。
3. 更新代码并重新构建镜像（本版本不发布镜像，见[安装部署](./02-installation.md)）：

   ```bash
   git pull
   ./scripts/build_frontend_dist.sh
   docker compose --profile docs up -d --build   # 带上你实际启用的 profile
   ```

   用 `scripts/deploy.sh` 部署的，重新执行同一个脚本即可，它会重新构建镜像并原地更新容器。
4. **看启动日志**：`docker compose logs -f app`。迁移在服务开始监听之前执行，日志里有 `[core] Current migration version: …`、`Database migrated from version A to B`。
5. **验证**：`curl localhost:8080/ready` 返回 200，「设置 → 版本信息」页的数据库版本（当前迁移版本号）已更新。
6. 出问题，且一时修不好：停服务，**恢复备份并换回旧版本的代码和镜像**。不要靠 `migrate down` 回退。

### 迁移失败时如何读启动报错

迁移默认失败即终止启动（`MIGRATION_FAIL_FAST=true`），报错包含三部分：

```
database migration failed, refusing to start against a half-migrated schema
(the database is at version 121 (dirty: true)): <golang-migrate 的原始错误，含出错的 SQL 行>

Next steps: run ./scripts/migrate.sh version ...
```

- **版本号 N + `dirty: true`**：第 N 号迁移执行到一半失败了。迁移没有被包在事务里，它前面的语句已经生效。
- **原始错误**：数据库返回的报错和出错的行号，去 `migrations/versioned/<N>_*.up.sql` 里对照。
- 出错的如果是扩展注册的迁移源，报错会写明 `migration source "<名字>"`，它有自己的版本记录表。

之后按 [docs/migration-troubleshooting.md](https://github.com/magicyuan876/yuheng/blob/main/docs/migration-troubleshooting.md) 的步骤处理：先修好根因（缺扩展、权限、磁盘），手工收拾半成品，再 `./scripts/migrate.sh force <上一个版本>` 把版本记录退回到上一个**实际存在**的迁移（编号有空段：000120 的上一个是 89，不是 119；启动报错里会直接给出这个版本号），最后重启。

两个相关开关：

| 变量 | 默认 | 含义 |
| --- | --- | --- |
| `MIGRATION_FAIL_FAST` | `true` | 迁移失败即终止启动。设为 `false` 时只告警并继续启动，仅适用于你在服务外自己迁移（CI、DBA）的部署；此时 `/ready` 仍返回 503，直到迁移状态正常。`AUTO_MIGRATE=false` 则完全不在启动时迁移 |
| `AUTO_RECOVER_DIRTY` | `false` | 遇到 dirty 状态自动回退一个版本并重跑迁移。**只有被中断的那个迁移可以安全地执行两遍时才能开**（每条语句都带 `IF NOT EXISTS` 之类的保护）；否则重跑会再次失败，或悄悄把数据改两遍。默认关，让你先看清再动手 |

### 就绪与存活探针

- `GET /health`：进程存活，不检查任何依赖。用于「进程是否需要重启」。
- `GET /ready`：数据库可达、（配置了 Redis 时）Redis 可达、迁移状态干净时返回 200，否则 503 和一个不含内部细节的 JSON。用于「是否该给它流量」。compose 的健康检查和 Helm 的 `readinessProbe` 指向它。

## 已发布的破坏性迁移

下表来自对 `migrations/versioned/*.up.sql` 里 `DROP TABLE` / `DROP COLUMN` / `DELETE` 的逐条核对。「后果」一栏说的是升级之后、以及想回到旧版本时会发生什么。

| 版本 | 删除了什么 | 后果 |
| --- | --- | --- |
| 000001 | 列 `knowledge_bases.rerank_model_id`（重排模型改为会话级设置） | 每个知识库上曾配置的重排模型丢失，需在会话里重新选择 |
| 000004 | 列 `knowledge_bases.vlm_model_id`（并入 `vlm_config` JSON） | 迁移本身不拷贝这个值；旧列里的数据丢失 |
| 000063 | 列 `knowledges.tag_id` | 先拷贝进关联表 `knowledge_tag_relations` 再删，数据保留；旧版本读不到标签 |
| 000065 | 列 `tenants.api_key` | 先拷贝进 `tenant_api_keys` 再删，数据保留；旧版本无法认证 |
| 000077 | 表 `wiki_log_entries`；`wiki_pages` 中 `page_type='log'` 的行 | Wiki 操作日志永久丢失（知识库活动记录是权威历史） |
| 000089 | 表 `custom_agents`、`agent_shares`、`tenant_disabled_shared_agents`、`mcp_tool_approvals`、`memory_*`（6 张）、`tenant_skills`、`tenant_skill_snapshots`、`im_channels`、`im_channel_sessions`、`embed_channels`；`sessions.agent_id`、`messages.agent_id` / `agent_tenant_id` / `agent_duration_ms`、`message_suggestion_sets.agent_id` / `agent_tenant_id`；并把 `sessions.agent_config` 改名为 `last_request_state`、`messages.agent_steps` 改名为 `turn_steps` | 自定义智能体、长期记忆、技能、IM 渠道绑定、嵌入渠道**全部丢失且不迁移**。其 `.down.sql` 只重建空表骨架，数据回不来；从 000089 往下回滚的路径也已不完整 |
| 000121 | 列 `knowledge_bases.cos_config` | 先把存储 provider 拷进 `storage_provider_config`；列里遗留的腾讯云 COS 凭据被删除（不再有代码读取） |
| 000123 | 列 `docs_pages.status`（`draft` / `published`） | 被 `exclude_from_knowledge` 取代：原来的草稿页变为「排除出知识库」，已发布页不变。旧版本无 `status` 语义 |
| 000134 | 列 `storage_backends.legacy_alias`；每个空间的环境存储副本与别名记录（`legacy_alias` 或 `source = env` 的行） | 换成一条全平台共享的部署存储记录 `env`；原来指向副本的空间默认、知识库、文档空间绑定与资源记录都改指 `env`。空间注册的别名配置（来自旧的空间存储配置）随之删除。本版本面向重建部署，没有为旧数据保留兼容 |
| 000135 | 列 `tenants.storage_engine_config` | 空间级的存储配置（含明文 S3 密钥）删除，存储只按存储后端实例配置 |
| 000136 | 列 `knowledge_bases.storage_provider_config` | 知识库只按 `storage_backend_id` 绑定存储后端 |
| 000137 | 列 `resources.provider`；索引 `idx_resources_tenant_location` | 文件位置只记后端 ID 与后端内位置，位置按后端唯一；旧版本写入的位置哈希不再参与去重 |

另外：000044 的 `.down.sql` 会 `DROP TABLE audit_logs`，为防止误用，它要求会话里显式设置 `yuheng.allow_destructive_migration = 'true'` 才会执行。

新增迁移时请遵守同样的标准：破坏性变更要在提交说明和 CHANGELOG 里写明，并更新这张表。

## 单副本与多副本

**默认按单副本 `app` 部署。** 要跑多个 `app` 副本，必须同时满足：

- **文件存储用共享的 S3**（`STORAGE_TYPE=s3`，RustFS 单实例也算；外部对象存储更佳）。`STORAGE_TYPE=local` 时文件在某个副本的本地卷里，其他副本读不到。
- **配置 Redis**（`REDIS_ADDR`）。事件、缓存和 Asynq 任务队列靠它在副本间共享；没有 Redis 时应用会在日志里提示按单进程运行。
- `JWT_SECRET`、`SYSTEM_AES_KEY` 等密钥所有副本一致。
- 数据库连接：每个副本最多占 `DB_MAX_OPEN_CONNS`（默认 50）个连接。`副本数 × DB_MAX_OPEN_CONNS + 其他客户端`必须小于 postgres 的 `max_connections`（compose 默认 100，可用 `POSTGRES_MAX_CONNECTIONS` 调），否则调小前者或调大后者。
- 迁移：多个副本同时启动时，迁移靠 postgres 的 advisory lock 串行执行，只有一个真正迁移，其余等待后发现已最新。

**当前的限制：** 数据源同步、审计日志保留清理、在线文档清理、知识健康的复核巡检这几类定时任务**在每个副本上各跑一份**，没有选主。它们对重复执行是安全的（复核巡检安排的重复检测会被合并成一次），但会白白多做工作并多占资源；数据源同步在多副本下会对同一个外部数据源重复拉取。

协同服务 `collab` 同理：单实例无需 Redis；多实例必须设置 `COLLAB_REDIS_URL`。

Helm 部署里对应的是 `app.replicaCount`，并遵守上面同样的前提。
