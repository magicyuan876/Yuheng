#!/usr/bin/env bash
# 探测到某个主机的实际可用 MTU，用于设置 .env 里的 DOCKER_NETWORK_MTU。
#
# 为什么需要它：容器默认按 MTU 1500 发包。如果目标服务在 VPN / 隧道后面，
# 而那条路径的 MTU 更小（常见 1400 左右），超长的包会被直接丢弃，且隧道通常
# 屏蔽了 ICMP "需要分片" 通知，TCP 无从得知、只能不断重传——表现是小请求正常、
# 大文件上传速度掉到几十 KB/s。用 DF（禁止分片）标志的 ping 逐档试探即可找出真值。
#
# 用法: ./scripts/probe-mtu.sh <host> [more hosts...]
#   例: ./scripts/probe-mtu.sh 10.52.2.213
set -uo pipefail

if [ $# -lt 1 ]; then
    echo "用法: $0 <host> [host...]" >&2
    echo "  例: $0 10.52.2.213" >&2
    exit 1
fi

# ICMP + IP 头 28 字节：payload N 对应 MTU N+28。
readonly HDR=28
# 从大到小试探，覆盖常见的以太网 / PPPoE / 各类隧道封装。
readonly SIZES="1472 1464 1452 1442 1422 1412 1392 1372 1352 1332 1272 1172"

# ping_df 在禁止分片的前提下发一个指定 payload 大小的包。
# 各平台参数不同：Linux 用 -M do，macOS/BSD 用 -D。
ping_df() {
    local host="$1" size="$2"
    if ping -c1 -W2 -M do -s "$size" "$host" >/dev/null 2>&1; then
        return 0
    fi
    # macOS / BSD 回退
    ping -c1 -t2 -D -s "$size" "$host" >/dev/null 2>&1
}

overall_min=""

for host in "$@"; do
    echo "== $host =="
    if ! ping -c1 -W2 "$host" >/dev/null 2>&1 && ! ping -c1 -t2 "$host" >/dev/null 2>&1; then
        echo "  不可达（ICMP 被屏蔽或主机不通），跳过"
        echo
        continue
    fi

    found=""
    for size in $SIZES; do
        if ping_df "$host" "$size"; then
            found=$((size + HDR))
            echo "  payload ${size} 通过  ->  路径 MTU = ${found}"
            break
        fi
        echo "  payload ${size} 失败"
    done

    if [ -z "$found" ]; then
        echo "  1172 以下都不通：可能整体屏蔽了 ICMP，无法用此方法探测"
    elif [ -z "$overall_min" ] || [ "$found" -lt "$overall_min" ]; then
        overall_min="$found"
    fi
    echo
done

if [ -z "$overall_min" ]; then
    echo "未能探测出 MTU。若目标屏蔽 ICMP，可保持默认 1500，或按经验值试 1400。"
    exit 0
fi

echo "所有目标中最小的路径 MTU: ${overall_min}"
if [ "$overall_min" -ge 1500 ]; then
    echo "已是标准值，无需设置 DOCKER_NETWORK_MTU。"
else
    echo
    echo "建议在 .env 中设置："
    echo "  DOCKER_NETWORK_MTU=${overall_min}"
    echo
    echo "然后重新执行部署脚本（会自动重建网络使其生效）。"
fi
