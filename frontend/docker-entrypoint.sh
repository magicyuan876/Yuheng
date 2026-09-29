#!/bin/sh

# Only emit whitelisted locale tags to avoid config.js injection from env values.
RUNTIME_DEFAULT_LOCALE=""
case "${DEFAULT_LOCALE:-}" in
  zh-CN|en-US|ru-RU|ko-KR) RUNTIME_DEFAULT_LOCALE="${DEFAULT_LOCALE}" ;;
esac

# ENTERPRISE_INFO_URL ends up inside a JS string literal, so accept only an absolute
# http(s) URL made of plain URL characters (no quotes, backslashes, spaces, or shell
# and HTML metacharacters). Anything else is dropped rather than escaped.
RUNTIME_ENTERPRISE_INFO_URL=""
case "${ENTERPRISE_INFO_URL:-}" in
  http://*|https://*)
    if printf '%s' "${ENTERPRISE_INFO_URL}" | grep -Eq '^[A-Za-z0-9._~:/?#@!&()*+,;=%-]+$'; then
      RUNTIME_ENTERPRISE_INFO_URL="${ENTERPRISE_INFO_URL}"
    fi
    ;;
esac

# Emitted as a JS boolean literal, never as raw text from the environment.
RUNTIME_HIDE_ENTERPRISE_PROMOTION="false"
case "${HIDE_ENTERPRISE_PROMOTION:-}" in
  true|TRUE|True|1) RUNTIME_HIDE_ENTERPRISE_PROMOTION="true" ;;
esac

# 生成运行时配置文件，注入环境变量到前端
cat > /usr/share/nginx/html/config.js << EOF
window.__RUNTIME_CONFIG__ = {
  MAX_FILE_SIZE_MB: ${MAX_FILE_SIZE_MB:-50},
  MAX_VIDEO_FILE_SIZE_MB: ${MAX_VIDEO_FILE_SIZE_MB:-2048},
  DEFAULT_LOCALE: "${RUNTIME_DEFAULT_LOCALE}",
  ENTERPRISE_INFO_URL: "${RUNTIME_ENTERPRISE_INFO_URL}",
  HIDE_ENTERPRISE_PROMOTION: ${RUNTIME_HIDE_ENTERPRISE_PROMOTION}
};
EOF

# 处理 nginx 配置
# client_max_body_size 取普通文件与视频上限的较大值（视频默认 2048MB）
DOC_LIMIT=${MAX_FILE_SIZE_MB:-50}
VIDEO_LIMIT=${MAX_VIDEO_FILE_SIZE_MB:-2048}
BODY_LIMIT=$DOC_LIMIT
[ "$VIDEO_LIMIT" -gt "$BODY_LIMIT" ] 2>/dev/null && BODY_LIMIT=$VIDEO_LIMIT
export MAX_FILE_SIZE=${BODY_LIMIT}M
export APP_HOST=${APP_HOST:-app}
export APP_PORT=${APP_PORT:-8080}
export APP_SCHEME=${APP_SCHEME:-http}
# 在线文档的协同服务。默认指向 compose 里的 collab 容器；该服务没启动时
# /collab 会返回 502，而这正是前端需要的答案 —— 连不上就退回独占编辑，
# 不会卡在连接中。
export COLLAB_HOST=${COLLAB_HOST:-collab}
export COLLAB_PORT=${COLLAB_PORT:-1234}
# 运行时解析 /collab 上游用的 DNS 服务器（见 nginx.conf）。默认取容器自己的
# resolv.conf：Docker 里是内置的 127.0.0.11，Kubernetes 里是集群 DNS。
if [ -z "${DNS_RESOLVER:-}" ]; then
  DNS_RESOLVER=$(awk '/^nameserver/ {print $2; exit}' /etc/resolv.conf 2>/dev/null)
fi
DNS_RESOLVER=${DNS_RESOLVER:-127.0.0.11}
case "${DNS_RESOLVER}" in
  *:*) DNS_RESOLVER="[${DNS_RESOLVER}]" ;; # IPv6 地址在 nginx 里要加方括号
esac
export DNS_RESOLVER
envsubst '${MAX_FILE_SIZE} ${APP_HOST} ${APP_PORT} ${APP_SCHEME} ${COLLAB_HOST} ${COLLAB_PORT} ${DNS_RESOLVER}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf

# 启动 nginx
exec nginx -g 'daemon off;'
