#!/usr/bin/env bash
# 一键部署（海外服务器版）：全部依赖走官方源。
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/deploy.sh" us
