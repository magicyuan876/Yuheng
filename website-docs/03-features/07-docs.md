# 在线文档

在线文档是 Yuheng 里的团队写作空间：按**空间**组织页面树，多人实时协同编辑（或在没有协同服务的部署里一人一次地编辑），带评论、@提及、关注通知、版本历史、模板、标签、搜索和公开分享。空间可以**绑定一个知识库**：页面会以 Markdown 自动同步进去，参与检索问答和[知识健康](22-knowledge-health.md)检测；受限页面永远不会进入知识库。

模块默认关闭。它由 Go 后端的 `internal/docs`、Node 写的协同服务 `collab/`（Hocuspocus / Yjs）和可选的自托管 draw.io 组成，前端在 `frontend/src/views/docs`。

## 1. 开启与部署

### 1.1 两种编辑模式

| | 只开在线文档 | 在线文档 + 协同服务 |
| --- | --- | --- |
| 空间、页面树、权限、评论、历史、标签、搜索、导入导出、知识库同步 | 可用 | 可用 |
| 同一页面多人同时编辑 | 否，**独占编辑** | 是，实时协同（光标、合并编辑） |
| 额外容器 | 无 | `collab` |

**独占编辑**不是降级版本：前后端是同一个编辑器、同一份构建产物，只有传输层不同。进入编辑时取得一个 5 分钟的租约，浏览器每分钟续期（保存也算续期）；其他人只读，并看到「某某正在编辑」。持有者关闭页面或断线满 5 分钟，租约过期，下一个人即可接管，不需要管理员解锁。这两个数值是代码常量，不可配置。

浏览器用哪种模式，只由部署能力 `docs_collab_url`（即 `YUHENG_COLLAB_URL`）决定：为空走独占编辑，非空走 WebSocket。填了地址而协同服务没有起来时，前端**不会**自动退回独占编辑，编辑器会连不上；服务端的租约接口在这种部署下一律返回 409。

### 1.2 Docker Compose

**第一步：打开模块。** `.env` 里设置：

```bash
YUHENG_DOCS_ENABLED=true
```

重启 app 后注册 `/api/v1/docs/**` 与 `/api/v1/groups/**` 路由，前端出现「文档」入口（前端按部署能力 `docs` 决定是否注册这组路由）。只做这一步就是独占编辑模式，不需要新容器。

**第二步（可选）：协同编辑。** `.env` 里再设置：

```bash
# 至少 16 个字符；app 和 collab 读的是同一项
YUHENG_COLLAB_SHARED_SECRET=$(openssl rand -base64 32)
# 浏览器实际访问的地址：前端 nginx 已把 /collab 代理到协同容器
YUHENG_COLLAB_URL=ws://<站点地址>/collab
```

`<站点地址>` 要带上前端端口（`FRONTEND_PORT` 不是 80 时，例如 `ws://localhost:8086/collab`）。HTTPS 站点填 `wss://<域名>/collab`，在 HTTPS 页面里填 `ws://` 会被浏览器当作混合内容拒绝。app 回调协同服务用容器名直连，compose 已为 app 设置 `YUHENG_COLLAB_INTERNAL_URL=http://collab:1234`。

协同服务的镜像不在 CI 发布，也不在 `make build-images` 里，需要先构建：

```bash
make build-images-collab        # 或 docker compose --profile docs build collab
docker compose --profile docs up -d
```

`docs` profile 启动 `collab` 和 `drawio` 两个容器（`collab` 等 app 与 redis 健康后才启动）。profile 只管容器，不会替你打开 `YUHENG_DOCS_ENABLED`。

**第三步（可选）：draw.io 图表。** 设置 `YUHENG_DOCS_DRAWIO_URL`。draw.io 以 iframe 嵌在编辑器里、由浏览器直接加载，所以要填**浏览器能打开**的地址，本机部署是 `http://localhost:8087/`，不能填 `http://drawio:8080`。drawio 容器默认只绑定 127.0.0.1（`DRAWIO_BIND`），给远程用户使用时改绑或放到反向代理后面。留空时不能新建、编辑 draw.io 图表，已有图表按保存时的预览图显示。白板（Excalidraw）打包在前端里，不需要任何服务。

### 1.3 端口与验证

| 服务 | 默认端口 | 说明 |
| --- | --- | --- |
| collab | `127.0.0.1:1234`（`COLLAB_BIND`、`COLLAB_PORT`） | 仅供调试；浏览器走前端的 `/collab` |
| drawio | `127.0.0.1:8087`（`DRAWIO_BIND`、`DRAWIO_PORT`） | 浏览器直接访问 |

```bash
curl -s http://localhost:1234/healthz
# 200 {"status":"ok","backend":"ok","connections":0,"documents":0,"dirty_documents":0,"version":"…"}
```

`backend` 为 `unreachable` 时返回 503、`status` 为 `degraded`：协同服务连不上 app，检查 `COLLAB_BACKEND_URL` 和两边的共享密钥。镜像的 HEALTHCHECK 用的就是这个接口，所以这种情况下容器会被判为不健康。

前端 nginx 的 `/collab` 只透传 WebSocket；协同服务的回调接口是 app 上的 `/internal/collab/*`，只在设置了共享密钥时注册，用 HMAC 签名校验，不经过用户会话。协同服务本身不连数据库，鉴权、加载、落盘都回调 app。

Helm 部署见 `helm/README.md`（`docs.enabled`、`collab.enabled`、`secrets.collabSharedSecret`；多副本必须 `collab.redis.enabled=true`）。chart 不包含 draw.io。

## 2. 空间与页面

### 2.1 空间

空间是页面的容器，有名称、路径标识（slug，工作区内唯一）、图标和描述，还有**可见性**：

| 可见性 | 谁能进 |
| --- | --- |
| `private` 私有 | 只有空间成员 |
| `open` 开放 | 工作区所有成员，按空间的**默认角色**（reader 或 writer）访问 |
| `public` 公开 | 同上，另外匿名访客可以只读（见 4.7）。需要部署开启 `YUHENG_DOCS_PUBLIC_SHARING`，否则拒绝设置 |

创建空间需要工作区 Contributor 及以上，创建者成为空间管理员。空间成员可以是用户或**用户组**，角色是 `reader`（只读、评论）、`writer`（编辑）、`admin`（管理成员、设置、回收站）。一个空间必须始终保留至少一个管理员。

用户组是工作区级的能力（在线文档是第一个使用者），在「设置 → 用户组」里管理，只有工作区管理员可以增删改；每个工作区有一个默认组，所有成员隐式属于它。

### 2.2 权限怎么算

一个人对某个页面的有效角色按顺序算出：

1. **工作区角色**：工作区 Owner 和 Admin 在本工作区的所有空间里都是空间管理员（包括私有空间）；Viewer 在任何地方最多是 reader；不是工作区的活跃成员则什么都没有。
2. **空间角色**：本人与所在组的成员身份中最强的一个；开放、公开空间再与默认角色取较强者。
3. **页面限制**：从根到该页面，每一个「受限」的祖先（含自身）都必须给他授权，且每一层只能**收窄**角色。空间管理员不受这一层约束。
4. **锁定**：锁定的页面对空间管理员以外的人一律最多 reader。

看不到的空间和页面一律返回 404，而不是 403。权限判定结果带缓存（`YUHENG_DOCS_ACL_CACHE_TTL_SECONDS`，默认 60 秒），任何成员、组、授权、页面树的变化都会立即让整个工作区的缓存失效。

除了上面的角色，路由还有一层工作区角色下限：读操作需要 Viewer，写操作需要 Contributor——但评论、收藏、关注、导出这类「读者就能做」的操作只要 Viewer。API Key 需要 `docs_read` / `docs_write` / `docs_admin` 能力（或完全访问），并以自己的身份访问：它能看到的是工作区给「所有人」的空间和页面，按能力封顶到 reader / writer；`docs_admin` 或完全访问时是所有空间的 admin（见 [API 参考](../04-api/02-api-docs.md#通用约定)）。

### 2.3 受限页面

在页面的「权限」面板里可以让页面**切断继承**，只列出的用户或组能看到它和它下面的整棵子树。

- 授权只是**上限**，不是提升：在空间里只是 reader 的人，在页面上被授予 admin 仍然是 reader。面板会显示每个人实际得到的角色，并标出在空间里根本没有角色、授权不起作用的人；
- 第一次限制时，页面作者自动被加入（admin，即「不收窄」），避免把写的人自己挡在外面；取消限制会连同授权列表一起清除；
- 修改页面权限需要页面上的 admin，由于授权不能提升到 admin，实际上就是空间管理员；
- 一个页面最多列 100 个用户或组，更多请用用户组；
- 能打开页面的人都能查看权限面板（「为什么同事看不到」不需要问管理员）。在上层就被限制时，面板会说明，但看不到的祖先页面不显示标题。

受限页面**不会进入知识库**，也**不能公开分享**：限制一个已入库或已分享的页面，会立即把它（及其子树）移出知识库，已有的公开链接随即失效。

### 2.4 锁定

空间管理员可以锁定页面：「这篇已经定稿，别再改」。锁定后除空间管理员外所有人最多只读，正在协同编辑的人会被立即断开编辑。只有空间管理员可以解锁。转交负责人不受锁定影响。

### 2.5 页面树与回收站

页面可以任意嵌套、拖拽排序（分数索引，移动只改一行），可以跨空间移动：跨空间移动时，移动者看不到的子页面留在原空间的根下，移动不会扩大任何页面的可见范围。复制页面会复制它和移动者能看到的子树（一次最多 2000 页），复制出的页面继承目标位置的权限，副本之间的页面链接会改指向副本。

删除页面会连同子树进入回收站，同时断开正在编辑的人、撤销指向它们的公开链接（恢复不会自动重新发布）。

| 操作 | 需要 |
| --- | --- |
| 查看回收站 | 空间 reader |
| 恢复页面 | 对该页面的 writer；父页面仍在回收站时恢复到空间根下 |
| 彻底删除一项 / 清空回收站 | 空间管理员 |

回收站保留 `YUHENG_DOCS_TRASH_RETENTION_DAYS` 天（默认 30），到期由后台清理任务连同附件、历史、评论、关注、通知、分享链接一起物理删除，审计日志里记为系统操作。

删除**空间**是软删除，其页面随之不可访问；只有工作区管理员能恢复空间（`POST /spaces/:sid/restore`，前端没有入口），slug 已被别的空间占用时需要先改名。目前没有任何清理任务会物理删除已删除的空间。

## 3. 编辑

### 3.1 协同编辑

有协同服务时，同一页面的所有人通过 WebSocket 编辑同一个 Yjs 文档：实时合并、显示他人光标和在线头像。Yjs 状态是正文的唯一真源，数据库里的 ProseMirror JSON 是协同服务落盘时生成的投影，渲染、搜索、历史、知识库同步都以它为输入。落盘有防抖（默认 2 秒，最长 10 秒）；浏览器还会在 IndexedDB 里缓存文档。锁定、删除页面会立即断开正在编辑的人；其它权限变化（例如限制页面）由协同服务每 5 分钟一次的权限复查生效。

导入、恢复历史版本、从知识库起草和 `PUT /pages/:pid/content` 都走同一条「替换正文」路径：有协同服务时作为一次 Yjs 事务应用到正在打开的文档上（在场的人立即看到，也能撤销）；没有时直接写 JSON 并清空 Yjs 状态，下一个打开的人从 JSON 重建。

单页 Yjs 状态上限 `YUHENG_DOCS_MAX_YDOC_BYTES`（默认 20 MiB），超出拒绝写入并提示拆分页面；compose 会把同一个值传给协同服务，两端必须一致。

### 3.2 编辑器能放什么

文档结构由 `packages/docs-schema/schema.json` 统一定义，前端编辑器和后端校验共用。前端输入 `/` 打开命令菜单。

| 类别 | 内容 |
| --- | --- |
| 文本 | 段落、标题、引用、分隔线、分页符；粗体、斜体、下划线、删除线、行内代码、链接、高亮、文字颜色、上下标 |
| 列表 | 无序、有序、任务列表 |
| 结构 | 表格（行列操作、表头）、提示框（callout）、多栏布局、折叠块、目录、子页面列表、脚注、状态标签 |
| 代码与公式 | 代码块、Mermaid 图、行内公式与公式块（LaTeX） |
| 图表 | draw.io 图表（需要 `YUHENG_DOCS_DRAWIO_URL`）、白板（Excalidraw，内置）。二者都以附件保存源文件和 SVG 预览 |
| 文件 | 图片、视频、音频、PDF 内嵌、其它附件：粘贴、拖入或选择文件即上传 |
| 引用 | 页面链接、@提及、块引用（见 3.4）、外部嵌入（见 3.3） |

编辑器还有查找替换、目录侧栏、字数统计、块拖动，粘贴 Markdown 会转换成对应结构。

**附件**存放在空间绑定的存储后端（未绑定时用工作区默认）。单个文件上限 `YUHENG_DOCS_MAX_ATTACHMENT_BYTES`（默认 200 MiB）。文件类型按内容嗅探、不信任扩展名；SVG 去掉脚本后才存；同一空间内容相同的文件只存一份、只计一次配额；下载时除图片、音视频、PDF 外一律作为附件下发。附件地址不是凭证：每次读取都重新检查对所属页面的读权限。上传而未被任何已保存页面引用的附件，24 小时后由清理任务删除。

**配额**：工作区的存储配额之外，每个空间还有自己的附件配额，默认 `YUHENG_DOCS_SPACE_QUOTA_BYTES`（0 为不限），只有工作区管理员能为单个空间修改；空间成员都能看到用量。上传必须同时装得下两者。

### 3.3 外部嵌入

嵌入（iframe）只允许白名单里的服务商，且**保存时**就校验，数据库里不会存下不该嵌的地址；iframe 地址由服务端按服务商规则重新生成，粘贴链接上的追踪参数不会带进去，只接受 https。内置服务商：youtube、bilibili、feishu、figma、loom、miro、canva。

- `YUHENG_DOCS_EMBED_PROVIDERS` 收窄内置列表（留空为全部，写错的名字被忽略）；
- `YUHENG_DOCS_EMBED_EXTRA_HOSTS` 追加自建服务的域名（以点开头包含子域名），这类地址按原样嵌入。

收窄白名单后，之前存下的嵌入在下次保存和下次公开访问时不再显示。

### 3.4 页面链接、块引用与反向链接

- **页面链接**按页面 ID 指向，改名、移动后仍然有效。页面侧栏显示**反向链接**（哪些页面链到这里），只列你能打开的页面；
- **块引用**（命令菜单里「复制块引用」，再粘贴到别的页面）只存「页面 ID + 块 ID」，显示的是源块**当前**的内容：一段定义被六个页面引用，只需要改一处。读者无权打开源页面时，引用和「块已删除」显示成同一个状态，不泄露源页面是否存在；
- 块引用的文字**不写进**引用页自己的正文文本。搜索会找到引用页，但只对能读源页面的人（见 4.6）。

### 3.5 模板

新建页面时可以从模板开始，也可以在页面上「存为模板」。模板分两种范围：

| 范围 | 谁能创建、修改、删除 | 谁能使用 |
| --- | --- | --- |
| 空间模板 | 创建、修改：该空间的 writer；删除：该空间的 admin | 该空间的成员 |
| 工作区模板 | 工作区管理员 | 所有成员，列表中排在前面 |

存为模板时会去掉无法带走的内容：页面链接、块引用、@提及和附件（附件属于上传它的空间）；前端在保存前会提示哪些内容会被去掉。每个范围最多 200 个模板。目前前端只有新建模板和使用模板，修改、删除模板只能通过 API。

### 3.6 从知识库材料起草

`POST /api/v1/docs/pages/:pid/draft` 用空间绑定知识库里**调用方指定的**若干条目（最多 20 条）起草页面正文：

- 不自动检索，材料由调用方点名，且必须属于该空间绑定的知识库；
- 用的是该知识库配置的摘要模型，没有单独的设置；知识库没有配置模型时不可用；
- 模型写 Markdown，再转换成文档结构；正文经「替换正文」路径写入（先留历史快照，可撤销），审计原因记为 `ai`，所用条目记在页面的 `source_refs` 里；
- 上传解析的文档只取它的**摘要**（description）作为材料，手工创建的知识取全文；尚未解析完的条目会报错；
- 材料总长截断到 24000 字、指令 2000 字；锁定的页面不能起草。

这一功能**目前只有 API，前端没有入口**。

### 3.7 版本历史

页面的每次保存都会按规则决定是否留下快照：

- 正文没变的保存从不留快照；
- 导入、恢复等有意的写入一定留快照；
- 新建的空白页面不留快照，第一次有内容时一定留；
- 自上次快照以来编辑者变了，立即留一份；
- 其余情况按时间间隔：页面创建后 5 分钟内每 1 分钟一份，之后每 5 分钟一份（从上一份快照算起，而不是停笔之后）。

旧快照会被压缩：最近 200 份原样保留，有意留下的快照永不删除，30 天内每天保留最后一份，更早的每月保留一份。

历史面板可以查看任一版本，与当前或另一版本比较：**文字比较**逐行显示增删（单次最多 2 万行），**结构比较**按块 ID 区分改写、删除、新增和移动。恢复需要页面 writer，走「替换正文」路径，恢复本身也会留下快照。

## 4. 协作

### 4.1 评论

评论可以锚定到选中的一段文字（记下被选中的原文，位置失效后仍能定位），也可以是整页评论；回复只有一层。

**只读成员也能评论**：评审一篇文档不应该需要编辑权。只有作者能编辑自己的评论；作者本人或页面 writer 可以解决讨论；作者本人或空间管理员可以删除。每个页面最多 1000 个讨论。

### 4.2 @提及

正文和评论里都可以 @人，候选列表只包含能读这个页面的人。被提及的人收到一条不合并、不受「静音」影响的通知，并自动开始关注该页面。

### 4.3 关注与通知

创建页面、评论、被提及会自动关注页面；也可以手动关注，或「静音」一个页面（保持关注但不再收到通知）。

| 事件 | 通知谁 | 合并 |
| --- | --- | --- |
| 页面上有新评论或回复 | 关注者、页面作者、被回复的人 | 同一页面 10 分钟内合并为一条 |
| 被 @提及（正文或评论） | 被提及的人 | 从不合并，静音也照样送达 |
| 页面正文变化 | **手动**关注的人（因评论、提及而自动关注的人不收） | 同一页面 1 小时内合并为一条 |
| 被授予页面或空间的访问权 | 被授权的人 | 从不合并 |
| 自己创建的公开链接访问量达到 10 / 50 / 100 / 500 / 1000 / 5000 / 10000 | 链接创建者 | 每个里程碑一次 |

已读的通知不会被合并进新事件；自己的操作不会通知自己；除提及外，静音的页面对作者和被回复的人同样生效。通知只在站内收件箱里（页面顶栏的通知中心），可以标记已读；**没有邮件通知**，代码里明确没有实现（部署没有邮件配置）。

页面、树、评论、权限的变化通过 SSE（`/api/v1/docs/events`）推给打开的页面，只作为「该刷新了」的提示。每个事件都按订阅者当前的权限过滤：看不到的页面，它的标题不会出现在事件里；通知只推给接收人（规则见 [API 参考](../04-api/02-api-docs.md#get-api-v1-docs-events)）。

### 4.4 收藏、最近编辑与空间首页

页面可以收藏（只读成员也可以）。空间首页显示最近编辑的页面（最多 20 个）、标签和你在这个空间的收藏，都只列你能打开的页面。「最近浏览」只存在浏览器本地，不跨设备。

### 4.5 标签

标签属于空间，有名称（最多 32 字）和颜色，每个空间最多 200 个、每个页面最多 20 个。writer 可以创建标签、给页面打标签；删除标签（会从所有页面上摘掉）需要空间管理员。空间首页可以按标签筛选页面。前端目前没有修改、删除标签的入口。

### 4.6 搜索

在线文档自己的搜索覆盖三类内容：页面标题与正文、评论、通过块引用显示的文字。

- 用**子串匹配**而不是 PostgreSQL 全文分词：`simple` 分词器不切中文，一句话会成为一个词，按词检索中文会什么都搜不到。装有 `pg_trgm` 扩展时有索引加速，没有时退化为 `ILIKE`；
- 查询 1–128 字，一个字也能搜（中文单字就是词），最多返回 50 条；
- 每条结果都按调用者的权限逐页过滤；块引用的命中按**源页面**的权限判断。

这与知识库的检索问答是两套东西：问答检索的是绑定知识库里的镜像（见第 6 节）。

### 4.7 公开分享与公开空间

两者都把内容放到公网，受同一个部署开关 `YUHENG_DOCS_PUBLIC_SHARING` 控制，默认关闭。关闭时公开路由根本不注册，已经发出的链接和公开空间立即失效，也不能把空间设为公开。

**公开链接**（页面「分享」面板）：

- 页面 writer 可以创建、修改、撤销；能读页面的人都能看到它有哪些公开链接；
- 可选：包含子页面、设置密码（4–64 字，每条链接 15 分钟内最多试 10 次，另有每 IP 每分钟 10 次的限制；验证通过后 12 小时内免输）、设置过期时间（最长一年）、允许搜索引擎收录；
- 每个页面最多 20 条有效链接；链接密钥 128 位随机数；
- 访客只拿到渲染后的正文和标题：没有评论、历史、周围的页面树和其他人的名字。正文里指向链接之外页面的链接显示为纯文本；
- 以下情况链接自动失效，不需要记得去撤销：页面或祖先被限制、页面进入回收站、过期、撤销。

**公开空间**：空间可见性设为 `public` 后，空间里所有未删除、未受限的页面匿名可读，地址用空间 ID（`/api/v1/docs/public-spaces/:sid`）。受限页面和回收站页面对访客不可见。

首次被访问和每个访问量里程碑都记入审计日志。

## 5. 导入与导出

**导入**（空间设置 → 导入）：上传单个 `.md` 或一个 `.zip`，需要空间 writer，可以指定导入到某个页面之下。

- 规则：每个 Markdown 文件是一个页面，含 Markdown 的文件夹也是一个页面；与同目录某个 `.md` 同名的文件夹是那个文件的子页面——这与导出的布局对应，所以导出再导入不会让树翻倍；
- 其它文件作为附件存入空间；包内页面之间的相对链接会变成页面链接（先建好所有空页面，再逐个写入内容）；
- 以发起人的身份逐个创建页面，放不到他本来放不了的地方，审计记录也是他；
- 单个文件转换失败不影响其余文件，任务结束为「部分完成」并列出跳过项；
- 限制：上传包 256 MiB，解压后总计 512 MiB、最多 5000 个文件，单个 Markdown 8 MiB；
- 导入是后台任务，前端轮询任务状态；服务重启时正在运行的任务会被标为失败，已创建的页面保留。

**导出**：

- 单个页面（页面菜单 → 导出）：Markdown（默认）或 HTML，立即返回文件。链接到你打不开的页面只显示占位文字而不是标题；块引用按屏幕上的同一权限规则展开；附件保留为指向本系统的地址（需要登录才能打开）；
- 整个空间（空间设置 → 导出）：只要空间 reader，打包成 zip，页面按树形排成文件和文件夹，页面之间的链接改写成相对路径。压缩包里**只有发起人当时能读的页面**，所以只有发起人能下载；最多 5000 页；24 小时后由清理任务连同任务记录删除。附件不打进压缩包。

## 6. 与知识库的联动

### 6.1 绑定

一个空间可以绑定一个知识库。绑定需要空间管理员（`PUT /api/v1/docs/spaces/:sid/knowledge-base`，或创建空间时传 `knowledge_base_id`），服务端只校验知识库属于同一工作区。**前端目前没有绑定入口**，空间设置里只显示已绑定的知识库 ID。

绑定后，空间里的页面会被逐步同步进知识库；改绑到另一个知识库时，镜像条目会移到新的知识库；解绑后镜像条目被移除。这些不需要手动操作，后台每 5 分钟一轮的复查会发现它们。工作区管理员也可以用 `POST /api/v1/docs/spaces/:sid/reindex` 立即重建（分页执行，返回每页的结果与跳过原因）。

### 6.2 同步什么、何时同步

每个页面在知识库里对应**一条**知识（渠道为 `docs`），内容是页面的标题和 Markdown 渲染结果（不是 JSON 也不是 HTML），以「已发布的手工知识」写入，因此照常分块、向量化并参与检索问答。编辑时更新同一条，不会重复创建。

以下页面不会入库，已入库的会被移除：

| 原因 | 说明 |
| --- | --- |
| `restricted` 受限 | 页面或任一祖先切断了继承。Yuheng 的检索没有逐条权限过滤，入库等于对所有能查询这个知识库的人公开 |
| `trashed` 在回收站 | |
| `excluded` 不参与知识库 | 页面上关闭了「参与知识库」，或已被取代（见 6.4） |
| `empty` 内容太少 | 正文不足 5 个字符 |
| `no-knowledge-base` | 空间没有绑定知识库 |

「不参与知识库」不是权限：能读页面的人照样能读，它只决定页面会不会出现在 AI 回答里（会议记录、草稿适合关掉）。页面 writer 可以切换；锁定的页面只有空间管理员能改。它取代了早期的「草稿 / 发布」状态（迁移 000123：原来的草稿被标为不参与知识库）。

同步是一个**持久化在数据库里的队列**（`docs_index_state`）：

- 编辑后等待 `YUHENG_DOCS_INDEX_DEBOUNCE_SECONDS`（默认 60 秒）再同步，连续保存只同步一次；限制、排除、移动、删除立即处理，并连带子树；
- 每个实例每 10 秒取一批到期页面，多副本之间用认领租约互斥，不会重复处理；重启不丢任务；
- 失败按 30 秒起翻倍退避重试，最长每小时重试一次，不会放弃；
- 记录上次发送内容的哈希，内容没变不重新向量化；
- 每 5 分钟复查一次漏掉的变化（例如绑定之前已存在的页面、空间设置的变化），已入库的页面至少每 24 小时复核一次。在知识库里手工删掉镜像条目，下次同步时会重新创建。

### 6.3 负责人与知识健康

每个页面有一个**负责人**：默认是创建者，当前负责人或页面管理员可以在页面上转交，新负责人必须能编辑该页面。负责人不是权限，而是知识健康把问题派给的人。负责人存在页面上，同步时复制到镜像条目上（镜像条目的负责人不能在知识库一侧修改）；最近一次修改正文的人记为镜像条目的最近经手人。

页面顶部会显示与本页有关的[知识健康](22-knowledge-health.md)问题。只有对方同样是你能打开的页面时才显示标题和证据，否则只显示「另有 N 条」。页面 writer 可以：

- **确认仍然有效**：重新开始复核计时（页面未入库时返回 409，没有可确认的东西）；
- **用本页取代它**：保留本页，把问题另一方的页面排除出知识库（需要能编辑两个页面）。

检测规则、派发规则和处理方式见[知识健康](22-knowledge-health.md)，这里不重复。

### 6.4 被取代的页面

页面被取代时**不删除**，而是排除出知识库，并记下取代它的文档（标题、条目、是页面时还有页面）、操作人和时间的快照。读者打开时顶部显示「本页已被《X》取代」，仍可阅读。有编辑权限的人重新打开「参与知识库」即撤销，取代标记随之清除。

## 7. 配置

完整注释见 `.env.example` 的 K 组。

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `YUHENG_DOCS_ENABLED` | `false` | 模块总开关 |
| `YUHENG_COLLAB_URL` | 空 | 浏览器侧协同 WebSocket 地址；空为独占编辑 |
| `YUHENG_COLLAB_SHARED_SECRET` | 空 | app 与协同服务之间的 HMAC 密钥；协同服务要求至少 16 个字符，否则拒绝启动。设置了 URL 却没有密钥时，app 启动时打印警告，内部回调接口不注册 |
| `YUHENG_COLLAB_INTERNAL_URL` | 由 `YUHENG_COLLAB_URL` 推导（ws→http、wss→https）；compose 中为 `http://collab:1234` | app 调用协同服务（替换正文、踢出连接）的地址 |
| `YUHENG_DOCS_MAX_YDOC_BYTES` | `20971520`（20 MiB） | 单页 Yjs 状态上限，compose 同时传给协同服务 |
| `YUHENG_DOCS_MAX_ATTACHMENT_BYTES` | `209715200`（200 MiB） | 单个附件上限 |
| `YUHENG_DOCS_SPACE_QUOTA_BYTES` | `0` | 没有单独设置配额的空间的附件上限，0 为不限 |
| `YUHENG_DOCS_TRASH_RETENTION_DAYS` | `30` | 回收站保留天数 |
| `YUHENG_DOCS_INDEX_DEBOUNCE_SECONDS` | `60` | 编辑后多久同步到知识库 |
| `YUHENG_DOCS_ACL_CACHE_TTL_SECONDS` | `60` | 权限判定缓存的最长有效期 |
| `YUHENG_DOCS_EMBED_PROVIDERS` | 空（全部内置） | 允许的内置嵌入服务商，逗号分隔 |
| `YUHENG_DOCS_EMBED_EXTRA_HOSTS` | 空 | 额外允许嵌入的域名 |
| `YUHENG_DOCS_DRAWIO_URL` | 空 | 浏览器可访问的 draw.io 地址；空则不能新建、编辑 draw.io 图表 |
| `YUHENG_DOCS_PUBLIC_SHARING` | `false` | 是否允许公开链接和公开空间 |
| `YUHENG_DOCS_CLEANUP_INTERVAL_MINUTES` | `60` | 后台清理（孤立附件、过期回收站、过期导出、中断的导入）的间隔 |

协同服务容器（compose 的 `collab`）另有：`COLLAB_BACKEND_URL`（默认 `http://app:8080`）、`COLLAB_REDIS_URL`（多实例必填，否则各实例上的同一文档互相看不见、后落盘的覆盖先落盘的）、`COLLAB_LOG_LEVEL`、`COLLAB_PORT`、`COLLAB_BIND`。draw.io 容器：`DRAWIO_PORT`（8087）、`DRAWIO_BIND`、`DRAWIO_VERSION`。

数值配置写错（非数字或负数）时打印警告并使用默认值，不会拒绝启动。

没有 Redis 的单进程部署同样可用：事件总线、权限缓存和幂等记录退化为进程内实现。多个 app 副本需要 Redis，否则事件和缓存失效不能跨实例传播。

## 8. API

所有路由在 `/api/v1` 下，只有 `YUHENG_DOCS_ENABLED=true` 时注册。「角色」指空间或页面上的有效角色；另有工作区角色下限（读 Viewer、写 Contributor，个别标注）。写操作支持 `Idempotency-Key` 请求头（租约与 Yjs 保存、上传、导入除外）。

**空间**

| 方法 | 路径 | 说明 | 角色 |
| --- | --- | --- | --- |
| GET / POST | `/docs/spaces` | 列出可见空间 / 创建 | 成员 / 成员（Contributor） |
| GET | `/docs/spaces/:sid`、`/docs/spaces/by-slug/:slug` | 空间详情 | reader |
| PATCH / DELETE | `/docs/spaces/:sid` | 修改 / 删除 | admin |
| POST | `/docs/spaces/:sid/restore` | 恢复已删除的空间 | 工作区管理员 |
| GET / PUT | `/docs/spaces/:sid/members` | 成员列表 / 添加或改角色 | reader / admin |
| DELETE | `/docs/spaces/:sid/members/:ptype/:pid` | 移除成员 | admin |
| PUT | `/docs/spaces/:sid/knowledge-base` | 绑定知识库与存储后端 | admin |
| POST | `/docs/spaces/:sid/reindex` | 重建知识库索引 | 工作区管理员 |
| GET / PUT | `/docs/spaces/:sid/usage`、`/docs/spaces/:sid/quota` | 附件用量 / 设置配额 | reader / 工作区管理员 |
| GET | `/docs/spaces/:sid/home`、`/tree`、`/pages-by-label` | 首页、页面树、按标签列页面 | reader |
| GET / POST / PATCH / DELETE | `/docs/spaces/:sid/labels[/:lid]` | 标签 | reader / writer / writer / admin |
| GET / DELETE | `/docs/spaces/:sid/trash`、`/trash/:pid` | 回收站 / 清空或彻底删除一项 | reader / admin |
| POST | `/docs/spaces/:sid/attachments` | 上传附件 | writer |
| POST | `/docs/spaces/:sid/imports`、`/export` | 导入 / 导出空间 | writer / reader |

**页面**

| 方法 | 路径 | 说明 | 角色 |
| --- | --- | --- | --- |
| POST | `/docs/pages` | 新建（可带模板或 Markdown） | 父页面或空间根的 writer |
| GET | `/docs/pages/:pid`、`/by-short-id/:short`、`/content` | 页面、正文 | reader |
| PUT | `/docs/pages/:pid/content` | 整体替换正文（JSON 或 Markdown） | writer |
| PATCH / DELETE | `/docs/pages/:pid` | 改标题、图标、封面 / 删除到回收站 | writer |
| POST | `/docs/pages/:pid/move`、`/duplicate`、`/restore` | 移动 / 复制 / 从回收站恢复 | writer / reader（目标需 writer）/ writer |
| GET | `/docs/pages/:pid/ancestors`、`/children`、`/backlinks`、`/attachments` | 祖先、子页面、反向链接、附件 | reader |
| GET | `/docs/pages/:pid/effective-permission`、`/access`、`/grants` | 我的权限、权限面板、授权列表 | reader |
| PUT / POST / DELETE | `/docs/pages/:pid/access`、`/grants[/:ptype/:principal]` | 限制或取消限制、增删授权 | admin |
| PUT | `/docs/pages/:pid/lock` | 锁定 / 解锁 | admin |
| PUT | `/docs/pages/:pid/knowledge` | 参与 / 不参与知识库 | writer |
| PUT / POST | `/docs/pages/:pid/owner`、`/review` | 转交负责人 / 确认仍然有效 | writer（转交限当前负责人或 admin） |
| GET / POST | `/docs/pages/:pid/findings`、`/supersede` | 知识健康问题 / 用本页取代另一方 | reader / writer（两页都需） |
| POST | `/docs/pages/:pid/draft` | 从知识库材料起草 | writer |
| GET | `/docs/pages/:pid/revisions[/:rid]`、`/revisions/:rid/diff` | 历史列表、版本、比较 | reader |
| POST | `/docs/pages/:pid/revisions/:rid/restore` | 恢复版本 | writer |
| GET / POST / PATCH / DELETE | `/docs/pages/:pid/comments[/:cid]`、`/comments/:cid/resolve` | 评论 | reader（作者、writer、admin 规则见 4.1） |
| GET / POST / PATCH / DELETE | `/docs/pages/:pid/shares[/:shid]` | 公开链接 | reader / writer |
| POST | `/docs/pages/:pid/export` | 导出单页 | reader |
| PUT | `/docs/pages/:pid/labels` | 设置页面标签 | writer |
| PUT | `/docs/pages/:pid/favourite`；GET / PUT `/watch`；PUT `/mute` | 收藏、关注、静音 | reader |
| GET | `/docs/pages/:pid/mention-candidates` | @提及候选人 | reader |
| GET / POST / DELETE | `/docs/pages/:pid/lease` | 独占编辑租约（仅无协同服务的部署，否则 409） | reader / writer |
| GET / PUT | `/docs/pages/:pid/ydoc` | 独占编辑模式下读写 Yjs 状态 | reader / writer |

**其它**

| 方法 | 路径 | 说明 | 角色 |
| --- | --- | --- | --- |
| GET | `/docs/events` | SSE 事件流，`?space=` / `?page=` 收窄 | 成员 |
| GET | `/docs/search` | 搜索页面、评论、块引用文字 | 成员（结果逐页过滤） |
| GET / POST | `/docs/notifications`、`/notifications/read`、`/notifications/archive` | 收件箱、标记已读、归档 | 成员（Viewer） |
| GET | `/docs/favourites` | 我在所有空间的收藏 | 成员 |
| GET / POST / PATCH / DELETE | `/docs/templates[/:tid]` | 模板；范围在服务里判断（见 3.5） | 成员 |
| GET / POST | `/docs/embeds/policy`、`/embeds/resolve` | 嵌入白名单与 draw.io 地址 / 校验一个嵌入地址 | 成员 |
| GET / POST | `/docs/page-links/suggest`、`/page-links/titles`、`/block-refs/resolve` | 页面链接建议、批量解析标题、解析块引用 | 成员（逐页过滤） |
| GET / DELETE | `/docs/attachments/:aid` | 下载 / 删除附件 | 成员（服务按所属页面判断） |
| GET | `/docs/imports/:jid`、`/exports/:jid`、`/exports/:jid/download` | 导入导出任务 | 成员（导出限发起人） |
| POST | `/docs/maintenance/orphan-attachments`、`/maintenance/expired-trash` | 手动运行清理，默认试运行，`dry_run=false` 才删除 | 工作区管理员 |
| GET / POST / PATCH / DELETE | `/groups[/:gid]`、`/groups/:gid/members[/:uid]` | 用户组 | 读：成员；写：工作区管理员 |

**匿名路由**（仅 `YUHENG_DOCS_PUBLIC_SHARING=true` 时注册，无需登录）：`GET /docs/public/:key`、`POST /docs/public/:key/unlock`、`GET /docs/public-spaces/:sid`、`GET /docs/public-spaces/:sid/pages/:short`。

## 9. 局限与已知约束

- **独占编辑**同一页面同一时刻只有一人能写；租约 5 分钟、续期 1 分钟是代码常量。
- **绑定知识库、从知识库起草、重建索引、恢复已删除空间**目前只有 API，前端没有入口；修改、删除模板和标签、归档通知也只能通过 API。
- 绑定知识库只检查它属于同一工作区，不检查绑定者能否编辑该知识库；镜像内容对所有能查询该知识库的人可见。
- 受限页面永远不进知识库。代码里已经能算出每个页面的可见主体（`acl.PageSubjects`），但检索管线还不支持按主体过滤，所以没有存储、也没有放开这条规则。
- 同步到知识库的 Markdown 里，块引用显示为「引用块不可用」占位，图表只有预览图链接，嵌入只剩链接。
- 公开链接和公开空间渲染出的 HTML 中，图片、附件和图表的地址指向需要登录的 `/api/v1/docs/attachments/:id`，代码中没有匿名的附件路由，匿名访客加载不到它们；块引用同样不展开。
- 起草时，上传解析的文档只以摘要作为材料。
- 没有邮件通知；「最近浏览」不跨设备。
- 搜索是子串匹配，大空间上的单字查询会是一次宽扫描。
- 已删除的空间不会被自动清理。
- `.env.example` 说 `YUHENG_DOCS_CLEANUP_INTERVAL_MINUTES` 取负数可关闭后台清理，但配置加载时负数被当作非法值回退为 60，实际关不掉；清理器本身支持非正数间隔关闭，只是环境变量到达不了。
- 各类上限：标题 500 字、正文 4 MiB、文档最多 20 万节点 / 64 层嵌套；页面授权 100 条；评论讨论 1000 个；标签每空间 200、每页 20；模板每范围 200；公开链接每页 20；导出 5000 页；导入 5000 个文件；复制子树 2000 页。

## 10. 实现参考

| 位置 | 内容 |
| --- | --- |
| `internal/docs/module.go` | 模块装配、可选依赖、索引器与清理器启动 |
| `internal/config/docs.go` | 全部 `YUHENG_DOCS_*` / `YUHENG_COLLAB_*` 配置 |
| `internal/router/routes_docs.go` | 路由、API Key 能力与角色要求 |
| `internal/docs/acl/` | 权限解析（工作区 → 空间 → 页面限制 → 锁定）与路由守卫 |
| `internal/docs/service/` | 业务逻辑：`page.go`、`space.go`、`access.go`、`lock.go`、`replace.go`、`lease.go`、`collab.go`、`history.go`、`comment.go`、`notify.go`、`share.go`、`publicspace.go`、`template.go`、`label.go`、`search.go`、`transclusion.go`、`attachment.go`、`quota.go`、`importjob.go`、`exportjob.go`、`export.go`、`draft.go`、`index.go`、`indexqueue.go`、`stewardship.go`、`supersede.go`、`findings.go`、`maintenance.go` |
| `internal/docs/index/policy.go` | 哪些页面入库 |
| `internal/docs/indexer.go`、`knowledgebridge.go`、`draftbridge.go` | 同步队列的事件入口，与知识库、模型服务的适配 |
| `internal/docs/cleanup.go` | 后台清理定时器 |
| `internal/docs/history/`、`notify/`、`share/`、`embed/`、`importer/`、`export/`、`draft/`、`search/`、`attachment/`、`comment/` | 各功能的纯函数规则 |
| `internal/docs/schema/`、`packages/docs-schema/schema.json` | 文档结构定义与校验 |
| `internal/docs/render/`、`internal/docs/markdown/` | 渲染（HTML / Markdown / 纯文本）与 Markdown 解析 |
| `collab/` | 协同服务（Hocuspocus），`src/config.ts` 为其环境变量 |
| `frontend/src/views/docs/`、`frontend/src/api/docs/` | 前端界面与 API 客户端 |
| `migrations/versioned/000120_docs_module.up.sql` | 模块表结构；后续 000122（同步队列）、000123（不参与知识库）、000125（负责人）、000127（取代标记） |
| `docker-compose.yml`（`collab`、`drawio`）、`frontend/nginx.conf`（`/collab`）、`helm/` | 部署 |
