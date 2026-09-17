#!/usr/bin/env bash
set -euo pipefail

#
# 本地测试 Homebrew Formula
#
# 流程：打包 → 创建本地 tap → 写入 Formula → brew install → 验证
#
# 用法:
#   ./scripts/test-homebrew.sh                    # 完整测试（含前端构建）
#   SKIP_FRONTEND=1 ./scripts/test-homebrew.sh    # 跳过前端构建
#   SKIP_BUILD=1 ./scripts/test-homebrew.sh       # 跳过构建（使用已有 tarball）
#

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

TAP_NAME="yuheng/test"
FORMULA_NAME="yuheng-lite-test"
VERSION="test"
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
ARCHIVE="Yuheng-lite_${VERSION}_${GOOS}_${GOARCH}"
TARBALL="${ROOT_DIR}/dist/${ARCHIVE}.tar.gz"

# ── Step 1: Build package ──
if [ "${SKIP_BUILD:-}" != "1" ]; then
    echo "=== Step 1: Build local package ==="
    SKIP_FRONTEND="${SKIP_FRONTEND:-0}" ./scripts/package-lite.sh "${VERSION}"
else
    echo "=== Step 1: Skipped (SKIP_BUILD=1) ==="
fi

if [ ! -f "${TARBALL}" ]; then
    echo "Error: tarball not found at ${TARBALL}"
    exit 1
fi

SHA=$(shasum -a 256 "${TARBALL}" | awk '{print $1}')
echo ""
echo "  Tarball: ${TARBALL}"
echo "  SHA256:  ${SHA}"

# ── Step 2: Create local tap ──
echo ""
echo "=== Step 2: Set up local tap ==="

# Create tap if it doesn't exist
TAP_DIR="$(brew --repository)/Library/Taps/yuheng/homebrew-test"
if [ ! -d "${TAP_DIR}" ]; then
    mkdir -p "${TAP_DIR}/Formula"
    (cd "${TAP_DIR}" && git init -q && git commit --allow-empty -m "init" -q)
    echo "  Created tap: ${TAP_NAME}"
else
    echo "  Tap already exists: ${TAP_NAME}"
fi

# ── Step 3: Write Formula into tap ──
echo ""
echo "=== Step 3: Generate Formula ==="

cat > "${TAP_DIR}/Formula/${FORMULA_NAME}.rb" << RUBY
class YuhengLiteTest < Formula
  desc "Yuheng Lite (local test)"
  homepage "https://github.com/magicyuan876/yuheng"
  version "${VERSION}"
  license "Apache-2.0"

  url "file://${TARBALL}"
  sha256 "${SHA}"

  def install
    libexec.install "Yuheng-lite"
    pkgshare.install "web" if File.directory?("web")
    pkgshare.install "config" if File.directory?("config")
    pkgshare.install ".env.lite.example"
    if File.directory?("migrations")
      pkgshare.install "migrations"
    end

    (bin/"yuheng-lite-test").write <<~SH
      #!/bin/bash
      CONFIG_DIR="\\\${YUHENG_CONFIG_DIR:-\\\${XDG_CONFIG_HOME:-\\\$HOME/.config}/yuheng-test}"
      DATA_DIR="\\\${YUHENG_DATA_DIR:-\\\${XDG_DATA_HOME:-\\\$HOME/.local/share}/yuheng-test}"

      mkdir -p "\\\$DATA_DIR/files" "\\\$CONFIG_DIR/config" 2>/dev/null

      if [ ! -f "\\\$CONFIG_DIR/config/config.yaml" ]; then
        cp -r "#{pkgshare}/config/" "\\\$CONFIG_DIR/config/"
      fi

      if [ ! -d "\\\$CONFIG_DIR/migrations" ] && [ -d "#{pkgshare}/migrations" ]; then
        ln -sf "#{pkgshare}/migrations" "\\\$CONFIG_DIR/migrations"
      fi

      if [ ! -f "\\\$CONFIG_DIR/.env.lite" ]; then
        cp "#{pkgshare}/.env.lite.example" "\\\$CONFIG_DIR/.env.lite"
        sed -i'' -e "s|DB_PATH=.*|DB_PATH=\\\$DATA_DIR/yuheng.db|" "\\\$CONFIG_DIR/.env.lite"
        sed -i'' -e "s|LOCAL_STORAGE_BASE_DIR=.*|LOCAL_STORAGE_BASE_DIR=\\\$DATA_DIR/files|" "\\\$CONFIG_DIR/.env.lite"
        echo ""
        echo "已创建配置文件: \\\$CONFIG_DIR/.env.lite"
        echo ""
      fi

      set -a
      source "\\\$CONFIG_DIR/.env.lite"
      set +a

      export DB_PATH="\\\${DB_PATH:-\\\$DATA_DIR/yuheng.db}"
      export LOCAL_STORAGE_BASE_DIR="\\\${LOCAL_STORAGE_BASE_DIR:-\\\$DATA_DIR/files}"
      export YUHENG_WEB_DIR="\\\${YUHENG_WEB_DIR:-#{pkgshare}/web}"

      cd "\\\$CONFIG_DIR"
      exec "#{libexec}/Yuheng-lite" "\\\$@"
    SH
  end

  def post_install
    (var/"yuheng-test").mkpath
    (var/"log").mkpath
  end

  service do
    run [bin/"yuheng-lite-test"]
    keep_alive true
    working_dir var/"yuheng-test"
    log_path var/"log/yuheng-lite-test.log"
    error_log_path var/"log/yuheng-lite-test.log"
  end

  test do
    assert_predicate bin/"yuheng-lite-test", :executable?
  end
end
RUBY

(cd "${TAP_DIR}" && git add -A && git commit -m "update formula" -q --allow-empty)
echo "  Formula written to: ${TAP_DIR}/Formula/${FORMULA_NAME}.rb"

# ── Step 4: Install ──
echo ""
echo "=== Step 4: Install ==="
brew reinstall "${TAP_NAME}/${FORMULA_NAME}" 2>&1 || brew install "${TAP_NAME}/${FORMULA_NAME}"

# ── Step 5: Verify ──
echo ""
echo "=== Step 5: Verify ==="
echo ""
echo "  which:"
which yuheng-lite-test || true
echo ""
echo "  Installed files:"
brew list "${TAP_NAME}/${FORMULA_NAME}"
echo ""
echo "  Test paths (isolated from production):"
echo "    Config: ~/.config/yuheng-test/.env.lite"
echo "    Data:   ~/.local/share/yuheng-test/"

echo ""
echo "=== Done ==="
echo ""
echo "前台运行:"
echo "  yuheng-lite-test"
echo ""
echo "后台服务:"
echo "  brew services start ${TAP_NAME}/${FORMULA_NAME}"
echo "  brew services info ${TAP_NAME}/${FORMULA_NAME}"
echo "  brew services stop ${TAP_NAME}/${FORMULA_NAME}"
echo ""
echo "日志:"
echo "  $(brew --prefix)/var/log/yuheng-lite-test.log"
echo ""
echo "卸载测试:"
echo "  brew services stop ${FORMULA_NAME} 2>/dev/null"
echo "  brew uninstall ${FORMULA_NAME}"
echo "  brew untap ${TAP_NAME}"
echo "  rm -rf ~/.config/yuheng-test ~/.local/share/yuheng-test"
