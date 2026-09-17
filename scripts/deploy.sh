#!/usr/bin/env bash
# Yuheng 一键源码部署脚本（全部核心服务：frontend / app / docreader / postgres / redis）
#
# 用法:
#   ./scripts/deploy.sh us    # 海外服务器：全部走官方源
#   ./scripts/deploy.sh cn    # 国内服务器：apt/Go/pip/npm 全部走国内镜像
#
# 或直接使用一键入口: ./scripts/deploy_us.sh / ./scripts/deploy_cn.sh
#
# 前置要求: git、docker（含 compose 插件）、Node.js >= 20（前端产物在宿主机构建）。
# 脚本幂等：重复执行会重新构建镜像并原地更新容器，数据卷不受影响。
set -euo pipefail

REGION="${1:-us}"
if [ "$REGION" != "us" ] && [ "$REGION" != "cn" ]; then
    echo "[ERROR] 未知区域 '$REGION'，用法: $0 [us|cn]" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

log()  { printf '\033[0;32m[DEPLOY]\033[0m %s\n' "$1"; }
warn() { printf '\033[1;33m[WARN]\033[0m %s\n' "$1"; }
die()  { printf '\033[0;31m[ERROR]\033[0m %s\n' "$1" >&2; exit 1; }

# ---------- 1. 环境检查 ----------
command -v docker >/dev/null 2>&1 || die "未安装 docker。Ubuntu: curl -fsSL https://get.docker.com | sh"
docker compose version >/dev/null 2>&1 || die "docker compose 插件不可用（需要 Docker Compose V2）"
docker info >/dev/null 2>&1 || die "docker daemon 未运行或当前用户无权限（试试 sudo usermod -aG docker \$USER 后重新登录）"

# Node.js 可选：宿主机没有（或版本过低）时改用 node:20 容器构建前端产物。
HOST_NODE_OK=0
if command -v node >/dev/null 2>&1; then
    NODE_MAJOR="$(node -v | sed 's/^v\([0-9]*\).*/\1/')"
    if [ "${NODE_MAJOR:-0}" -ge 20 ]; then
        HOST_NODE_OK=1
    fi
fi
if [ "$HOST_NODE_OK" = "1" ]; then
    log "环境检查通过: docker / compose / node $(node -v)"
else
    log "环境检查通过: docker / compose（宿主机无 Node.js >= 20，前端将在 Docker 容器中构建）"
fi

# ---------- 2. 区域镜像源 ----------
if [ "$REGION" = "cn" ]; then
    log "区域: 中国 —— 启用国内镜像源"
    # Dockerfile.app: sed 替换 deb.debian.org（不带协议）
    export APK_MIRROR_ARG="${APK_MIRROR_ARG:-mirrors.aliyun.com}"
    # Dockerfile.docreader: sed 替换 http://deb.debian.org（带协议）
    export APT_MIRROR="${APT_MIRROR:-http://mirrors.aliyun.com}"
    export GOPROXY_ARG="${GOPROXY_ARG:-https://goproxy.cn,direct}"
    export PIP_INDEX_URL="${PIP_INDEX_URL:-https://mirrors.aliyun.com/pypi/simple/}"
    # 前端宿主机构建走 npmmirror
    export npm_config_registry="${npm_config_registry:-https://registry.npmmirror.com}"
else
    log "区域: 海外 —— 全部使用官方源"
fi

# ---------- 3. .env 准备 ----------
if [ ! -f .env ]; then
    cp .env.example .env
    # 默认端口：Web UI 8088 / API 9527
    sed -i 's/^FRONTEND_PORT=.*/FRONTEND_PORT=8088/' .env
    sed -i 's/^APP_PORT=.*/APP_PORT=9527/' .env
    log "已从 .env.example 生成 .env（默认端口: UI 8088 / API 9527）"
else
    log "检测到已有 .env，保留现有配置"
fi

# 从 .env 读一个变量的值。
# 必须去掉 \r：.env.example 是 CRLF 行尾，据此生成的 .env 每行都带回车符，
# 否则取到的是 "8088\r"，拼进 URL 或做数值比较时都会出错。
env_value() {
    grep -E "^$1=" .env 2>/dev/null | head -1 | cut -d= -f2- | tr -d '\r' || true
}

FRONTEND_PORT_VALUE="$(env_value FRONTEND_PORT)"
FRONTEND_PORT_VALUE="${FRONTEND_PORT_VALUE:-8088}"

# ---------- 3b. 按机器规格自动调优 ----------
# 只处理「系统设置页改不了」的那部分：文档解析并发、Embedding 协程池、Postgres。
# 任务池（核心解析/内容富化/Wiki 等）在 系统设置 → 运行与并发 里调，脚本不碰——
# 那些值存在数据库里，优先级高于环境变量，脚本写了也不会生效。
#
# 已经写在 .env 里的生效值一律保留，只填注释掉的和缺失的，所以手工调过的值不会
# 被重复执行的部署覆盖。想让脚本重算某项，把 .env 里那行删掉或注释掉再跑。
#
# CPU/内存预算默认只取机器的一部分，给同机的其他服务（例如本地 GPU 推理）留余量。
# 需要独占时用 TUNE_CPU_PERCENT=100 TUNE_MEM_PERCENT=80 ./scripts/deploy.sh cn 覆盖。
TUNE_CPU_PERCENT="${TUNE_CPU_PERCENT:-60}"
TUNE_MEM_PERCENT="${TUNE_MEM_PERCENT:-50}"

# 写入一项配置：已有生效值则跳过；有注释行则就地取消注释；否则追加。
set_env_if_absent() {
    local key="$1" val="$2"
    if grep -qE "^${key}=" .env; then
        return 0
    fi
    if grep -qE "^# *${key}=" .env; then
        sed -i "s|^# *${key}=.*|${key}=${val}|" .env
    else
        printf '%s=%s\n' "$key" "$val" >> .env
    fi
    TUNED_LINES="${TUNED_LINES}  ${key}=${val}\n"
}

# 写入一项配置：仅当它仍等于 .env.example 的出厂默认值时才改写。
# 用于那些在 .env.example 里就已生效（非注释）的项——set_env_if_absent 对它们
# 永远不会动手，但出厂默认值往往是按最小机器定的，需要按规格抬上去。
# 用户只要改成过别的值，这里就不再干预。
set_env_if_default() {
    local key="$1" factory="$2" val="$3"
    local cur
    cur="$(env_value "$key")"
    if [ -z "$cur" ]; then
        set_env_if_absent "$key" "$val"
        return 0
    fi
    if [ "$cur" != "$factory" ]; then
        return 0
    fi
    sed -i "s|^${key}=.*|${key}=${val}|" .env
    TUNED_LINES="${TUNED_LINES}  ${key}=${val}  (出厂默认 ${factory})\n"
}

# 夹取到 [min,max] 区间，并保证至少为 1。
clamp() {
    local v="$1" lo="$2" hi="$3"
    [ "$v" -lt "$lo" ] && v="$lo"
    [ "$v" -gt "$hi" ] && v="$hi"
    printf '%s' "$v"
}

autotune_env() {
    local cores mem_gb budget_cores budget_mem
    cores="$(nproc 2>/dev/null || echo 0)"
    mem_gb="$(awk '/MemTotal/{print int($2/1024/1024)}' /proc/meminfo 2>/dev/null || echo 0)"

    if [ "${cores:-0}" -lt 1 ] || [ "${mem_gb:-0}" -lt 1 ]; then
        warn "无法探测 CPU/内存规格，跳过自动调优（保持默认值）"
        return 0
    fi

    budget_cores=$(( cores * TUNE_CPU_PERCENT / 100 ))
    budget_mem=$(( mem_gb * TUNE_MEM_PERCENT / 100 ))
    [ "$budget_cores" -lt 1 ] && budget_cores=1
    [ "$budget_mem" -lt 1 ] && budget_mem=1

    log "机器规格: ${cores} 核 / ${mem_gb}GB 内存；调优预算: ${TUNE_CPU_PERCENT}% CPU (${budget_cores} 核) / ${TUNE_MEM_PERCENT}% 内存 (${budget_mem}GB)"

    TUNED_LINES=""

    # --- 文档解析（docreader，纯本地 CPU，是批量入库的主要瓶颈）---
    # gRPC 线程多为 IO 等待（等子进程 / 外部解析器），可以略超预算核数。
    set_env_if_absent DOCREADER_GRPC_MAX_WORKERS "$(clamp $(( budget_cores )) 4 64)"
    # 以下三项是解析器内部线程池，与 gRPC 线程相乘，取值要保守得多。
    set_env_if_absent DOCREADER_PDF_RENDER_MAX_WORKERS "$(clamp $(( budget_cores / 4 )) 1 16)"
    set_env_if_absent DOCREADER_MARKITDOWN_MAX_WORKERS "$(clamp $(( budget_cores / 8 )) 1 8)"
    set_env_if_absent DOCREADER_ODL_MAX_WORKERS "$(clamp $(( budget_cores / 8 )) 1 8)"

    # --- Embedding 协程池 ---
    # 真正的上限来自模型服务（远程 API 配额或本地 GPU 显存），不是本机 CPU，
    # 所以给一个温和的值；本地 GPU 跑起来后按显存和利用率再往上调。
    set_env_if_default CONCURRENCY_POOL_SIZE 5 "$(clamp $(( budget_cores / 8 )) 5 32)"

    # --- Postgres（向量 + 全文检索都在这里）---
    # shared_buffers 取预算内存的 1/4；超过 32GB 后收益递减，官方亦不建议再加。
    local shared_buffers effective_cache maint_mem
    shared_buffers=$(clamp $(( budget_mem / 4 )) 1 32)
    effective_cache=$(clamp $(( budget_mem * 3 / 4 )) 1 256)
    # 建 HNSW 向量索引极吃 maintenance_work_mem，但它会被并行维护进程各占一份。
    maint_mem=$(clamp $(( budget_mem / 16 )) 1 8)

    set_env_if_absent POSTGRES_SHARED_BUFFERS "${shared_buffers}GB"
    set_env_if_absent POSTGRES_EFFECTIVE_CACHE_SIZE "${effective_cache}GB"
    set_env_if_absent POSTGRES_MAINTENANCE_WORK_MEM "${maint_mem}GB"
    # work_mem 是每个排序/哈希节点各占一份，会被并发数与并行度放大，故保持较小值。
    set_env_if_absent POSTGRES_WORK_MEM "$(clamp $(( budget_mem * 2 )) 4 128)MB"
    set_env_if_absent POSTGRES_MAX_CONNECTIONS "$(clamp $(( budget_cores * 4 )) 100 500)"
    set_env_if_absent POSTGRES_MAX_WORKER_PROCESSES "$(clamp "$budget_cores" 8 64)"
    set_env_if_absent POSTGRES_MAX_PARALLEL_WORKERS "$(clamp "$budget_cores" 8 64)"
    set_env_if_absent POSTGRES_MAX_PARALLEL_WORKERS_PER_GATHER "$(clamp $(( budget_cores / 8 )) 2 16)"
    set_env_if_absent POSTGRES_MAX_PARALLEL_MAINTENANCE_WORKERS "$(clamp $(( budget_cores / 8 )) 2 16)"
    # 批量导入时 WAL 写入密集，调大可显著减少 checkpoint 抖动。
    set_env_if_absent POSTGRES_MAX_WAL_SIZE "$(clamp $(( budget_mem / 8 )) 1 16)GB"

    if [ -n "$TUNED_LINES" ]; then
        log "已写入自动调优配置（仅补齐缺失项，不覆盖已有值）:"
        printf "%b" "$TUNED_LINES"
        warn "任务池并发（核心解析/内容富化/Wiki 等）请在 系统设置 → 运行与并发 中调整"
    else
        log "自动调优: .env 中相关项均已配置，保持不变"
    fi
}

autotune_env

# 把 APP_EXTERNAL_URL 写入 .env（覆盖已有行 / 取消注释 / 追加）。
set_external_url() {
    local url="$1"
    if grep -qE '^APP_EXTERNAL_URL=' .env; then
        sed -i "s|^APP_EXTERNAL_URL=.*|APP_EXTERNAL_URL=${url}|" .env
    elif grep -qE '^# *APP_EXTERNAL_URL=' .env; then
        sed -i "s|^# *APP_EXTERNAL_URL=.*|APP_EXTERNAL_URL=${url}|" .env
    else
        printf '\nAPP_EXTERNAL_URL=%s\n' "$url" >> .env
    fi
    log "APP_EXTERNAL_URL 已设置为 ${url}"
}

# APP_EXTERNAL_URL 只是生成资源外链（如 IM 图片）的前缀，不影响端口开放。
# 交互模式下列出候选 IP 让用户选择或手动输入；非交互（CI/管道）保留现状，
# 首次生成时兜底用第一个内网 IP。
CURRENT_EXTERNAL_URL="$(env_value APP_EXTERNAL_URL)"

# 候选 IP：内网网卡地址（过滤 docker 网桥常用的 172.17.x），再加公网出口 IP。
INTRANET_IPS="$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -vE '^$|^172\.17\.' || true)"
PUBLIC_IP="$(curl -fsS --max-time 5 https://ifconfig.me 2>/dev/null || true)"

if [ -t 0 ]; then
    echo ""
    echo "== 配置 APP_EXTERNAL_URL（用于生成 IM 图片等资源的外部链接，不影响端口开放）=="
    CANDIDATES=()
    while IFS= read -r ip; do
        [ -n "$ip" ] && CANDIDATES+=("http://${ip}:${FRONTEND_PORT_VALUE}|内网")
    done <<< "$INTRANET_IPS"
    if [ -n "$PUBLIC_IP" ]; then
        CANDIDATES+=("http://${PUBLIC_IP}:${FRONTEND_PORT_VALUE}|公网出口")
    fi
    i=1
    for entry in "${CANDIDATES[@]}"; do
        echo "  ${i}) ${entry%%|*}  (${entry##*|})"
        i=$((i + 1))
    done
    echo "  m) 手动输入完整地址（如 http://yuheng.example.com）"
    if [ -n "$CURRENT_EXTERNAL_URL" ]; then
        echo "  回车) 保留当前值: ${CURRENT_EXTERNAL_URL}"
    else
        echo "  回车) 暂不设置（之后可在 .env 中配置）"
    fi
    read -r -p "请选择 [1-$((i - 1))/m/回车]: " choice
    case "$choice" in
        "")
            log "APP_EXTERNAL_URL 保持不变: ${CURRENT_EXTERNAL_URL:-<未设置>}"
            ;;
        m|M)
            read -r -p "请输入完整地址: " manual_url
            if [ -n "$manual_url" ]; then
                set_external_url "$manual_url"
            else
                warn "输入为空，APP_EXTERNAL_URL 保持不变"
            fi
            ;;
        *)
            if [ "$choice" -ge 1 ] 2>/dev/null && [ "$choice" -lt "$i" ]; then
                selected="${CANDIDATES[$((choice - 1))]}"
                set_external_url "${selected%%|*}"
            else
                warn "无效选择，APP_EXTERNAL_URL 保持不变"
            fi
            ;;
    esac
    echo ""
elif [ -z "$CURRENT_EXTERNAL_URL" ]; then
    FIRST_IP="$(printf '%s\n' "$INTRANET_IPS" | head -1)"
    if [ -n "$FIRST_IP" ]; then
        set_external_url "http://${FIRST_IP}:${FRONTEND_PORT_VALUE}"
        warn "非交互模式：APP_EXTERNAL_URL 自动使用内网地址，如需调整请修改 .env"
    else
        warn "无法探测服务器地址，请手动在 .env 中设置 APP_EXTERNAL_URL"
    fi
fi

# ---------- 4. 前端产物 ----------
if [ "$HOST_NODE_OK" = "1" ]; then
    log "构建前端静态资源（宿主机 npm ci + build）..."
    ./scripts/build_frontend_dist.sh
else
    log "构建前端静态资源（node:20 容器内 npm ci + build）..."
    COMMIT_ID="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
    # 以当前用户身份运行，避免 dist/node_modules 变成 root 属主；
    # HOME/npm 缓存指到 /tmp，非 root 用户在容器里也能写。
    docker run --rm \
        -u "$(id -u):$(id -g)" \
        -e HOME=/tmp \
        -e npm_config_cache=/tmp/.npm \
        -e VITE_IS_DOCKER=true \
        -e VITE_FRONTEND_COMMIT="$COMMIT_ID" \
        ${npm_config_registry:+-e npm_config_registry="$npm_config_registry"} \
        -v "$PROJECT_ROOT/frontend:/src" \
        -w /src \
        node:20-bookworm \
        sh -c "npm ci && npm run build"
fi
[ -d frontend/dist ] || die "前端产物 frontend/dist 不存在，构建失败"

# ---------- 5. 构建镜像 ----------
log "构建 Docker 镜像（首次构建约 10-20 分钟）..."
docker compose build

# ---------- 6. 网络 MTU ----------
# Docker 只在创建网络时应用 MTU，`up -d` 不会改动已存在的网络。所以改了
# DOCKER_NETWORK_MTU 之后如果不重建，配置会静默失效——用户以为改了、实际没生效。
# 这里主动比对并在不一致时重建，避免那种查半天的坑。
DESIRED_MTU="$(grep -E '^DOCKER_NETWORK_MTU=' .env 2>/dev/null | head -1 | cut -d= -f2 | tr -d '"'"'"' \r' || true)"
DESIRED_MTU="${DESIRED_MTU:-1500}"
EXISTING_NET="$(docker network ls --filter name=Yuheng-network --format '{{.Name}}' 2>/dev/null | head -1)"
if [ -n "$EXISTING_NET" ]; then
    CURRENT_MTU="$(docker network inspect "$EXISTING_NET" \
        -f '{{index .Options "com.docker.network.driver.mtu"}}' 2>/dev/null || true)"
    CURRENT_MTU="${CURRENT_MTU:-1500}"
    if [ "$CURRENT_MTU" != "$DESIRED_MTU" ]; then
        warn "网络 MTU 需从 ${CURRENT_MTU} 变更为 ${DESIRED_MTU}；MTU 只能在创建网络时设置，正在停止服务以重建网络（数据卷不受影响）..."
        docker compose down
    fi
fi

# ---------- 7. 启动 ----------
log "启动全部核心服务（网络 MTU=${DESIRED_MTU}）..."
docker compose up -d

echo ""
log "部署完成！服务状态:"
docker compose ps
echo ""
APP_PORT_VALUE="$(env_value APP_PORT)"
log "Web UI:  http://<服务器地址>:${FRONTEND_PORT_VALUE:-8088}"
log "API:     http://<服务器地址>:${APP_PORT_VALUE:-9527}"
echo ""
warn "首次部署后请在 Web UI 中完成初始化："
warn "  1. 注册管理员账号"
warn "  2. 配置模型（LLM / Embedding / Rerank / ASR / VLM）——模型配置存在数据库里，不随代码迁移"
warn "  3. 配置数据源凭据（飞书 / Lark 等）"
if [ "$DESIRED_MTU" = "1500" ]; then
    echo ""
    warn "若要对接 VPN / 隧道后的服务（自建 MinerU / 天枢、内网模型或对象存储），"
    warn "先探测链路 MTU：./scripts/probe-mtu.sh <目标主机>"
    warn "小于 1500 时在 .env 设置 DOCKER_NETWORK_MTU 并重跑本脚本，否则大文件传输会极慢甚至超时。"
fi
echo ""
warn "常用命令: docker compose logs -f app | docker compose ps | docker compose down"
