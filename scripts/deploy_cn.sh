#!/usr/bin/env bash
# 一键部署（中国服务器版）：apt / Go / pip / npm 全部走国内镜像源。
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/deploy.sh" cn
