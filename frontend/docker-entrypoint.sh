#!/bin/sh

# Only emit whitelisted locale tags to avoid config.js injection from env values.
RUNTIME_DEFAULT_LOCALE=""
case "${DEFAULT_LOCALE:-}" in
  zh-CN|en-US|ru-RU|ko-KR) RUNTIME_DEFAULT_LOCALE="${DEFAULT_LOCALE}" ;;
esac

# 生成运行时配置文件，注入环境变量到前端
cat > /usr/share/nginx/html/config.js << EOF
window.__RUNTIME_CONFIG__ = {
  MAX_FILE_SIZE_MB: ${MAX_FILE_SIZE_MB:-50},
  MAX_VIDEO_FILE_SIZE_MB: ${MAX_VIDEO_FILE_SIZE_MB:-2048},
  DEFAULT_LOCALE: "${RUNTIME_DEFAULT_LOCALE}"
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
envsubst '${MAX_FILE_SIZE} ${APP_HOST} ${APP_PORT} ${APP_SCHEME} ${COLLAB_HOST} ${COLLAB_PORT}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf

# 启动 nginx
exec nginx -g 'daemon off;'
