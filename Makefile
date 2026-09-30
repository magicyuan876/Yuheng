.PHONY: help build run test fmt fmt-check lint clean docker-build-app docker-build-docreader docker-build-frontend docker-build-all docker-run clean-db migrate-up migrate-down docker-restart docker-stop start-all stop-all start-ollama stop-ollama build-images build-images-app build-images-docreader build-images-frontend build-images-collab clean-images check-env list-containers pull-images show-platform dev-start dev-stop dev-restart dev-logs dev-status dev-app dev-frontend docs install-swagger anydoc-lib build-anydoc

# Show help
help:
	@echo "Yuheng Makefile 帮助"
	@echo ""
	@echo "基础命令:"
	@echo "  build             构建应用"
	@echo "  run               运行应用"
	@echo "  test              运行测试"
	@echo "  anydoc-lib        构建 anydoc 静态库（需要 Rust 工具链）"
	@echo "  build-anydoc      构建带 anydoc 解析引擎的应用"
	@echo "  clean             清理构建文件"
	@echo ""
	@echo "Docker 命令:"
	@echo "  docker-build-app       构建应用 Docker 镜像 (magicyuan876/yuheng-app)"
	@echo "  docker-build-docreader 构建文档读取器镜像 (magicyuan876/yuheng-docreader)"
	@echo "  docker-build-frontend  构建前端镜像 (magicyuan876/yuheng-ui)"
	@echo "  docker-build-all       构建所有 Docker 镜像"
	@echo "  docker-run            运行 Docker 容器"
	@echo "  docker-stop           停止 Docker 容器"
	@echo "  docker-restart        重启 Docker 容器"
	@echo ""
	@echo "服务管理:"
	@echo "  start-all         启动所有服务"
	@echo "  stop-all          停止所有服务"
	@echo "  start-ollama      仅启动 Ollama 服务"
	@echo ""
	@echo "镜像构建:"
	@echo "  build-images      从源码构建所有镜像"
	@echo "  build-images-app  从源码构建应用镜像"
	@echo "  build-images-docreader 从源码构建文档读取器镜像"
	@echo "  build-images-frontend  从源码构建前端镜像"
	@echo "  build-images-collab    从源码构建在线文档的协同服务镜像"
	@echo "  clean-images      清理本地镜像"
	@echo ""
	@echo "数据库:"
	@echo "  migrate-up        执行数据库迁移"
	@echo "  migrate-down      回滚数据库迁移"
	@echo ""
	@echo "开发工具:"
	@echo "  fmt               格式化代码（gofumpt，整个仓库；缺工具时会给出安装命令）"
	@echo "  fmt-check         检查格式，有未格式化文件则失败（CI 同款）"
	@echo "  lint              代码检查"
	@echo "  deps              安装依赖"
	@echo "  docs              生成 Swagger API 文档"
	@echo "  install-swagger   安装 swag 工具"
	@echo ""
	@echo "环境检查:"
	@echo "  check-env         检查环境配置"
	@echo "  list-containers   列出运行中的容器"
	@echo "  pull-images       拉取第三方基础镜像（Yuheng 自身镜像在本机构建）"
	@echo "  show-platform     显示当前构建平台"
	@echo ""
	@echo "开发模式（推荐）:"
	@echo "  dev-start         启动开发环境基础设施（仅启动依赖服务）"
	@echo "                    可选: make dev-start DEV_ARGS=--odl-hybrid"
	@echo "  dev-stop          停止开发环境"
	@echo "  dev-restart       重启开发环境"
	@echo "  dev-logs          查看开发环境日志"
	@echo "  dev-status        查看开发环境状态"
	@echo "  dev-app           启动后端应用（本地运行，需先运行 dev-start）"
	@echo "                    已 make anydoc-lib 时自动链接 anydoc 引擎"
	@echo "  dev-frontend      启动前端（本地运行，需先运行 dev-start）"
	@echo ""

# Go related variables
BINARY_NAME=Yuheng
MAIN_PATH=./cmd/server

# Docker related variables
DOCKER_IMAGE=magicyuan876/yuheng-app
DOCKER_TAG=latest

# Platform detection
ifeq ($(shell uname -m),x86_64)
    PLATFORM=linux/amd64
else ifeq ($(shell uname -m),aarch64)
    PLATFORM=linux/arm64
else ifeq ($(shell uname -m),arm64)
    PLATFORM=linux/arm64
else
    PLATFORM=linux/amd64
endif

# Build the application
build:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

# Build the anydoc static archive (Rust) that the `anydoc` build tag links.
# Override the platform with TARGET=<rust-target-triple>.
anydoc-lib:
	./scripts/build-anydoc-lib.sh

# Build the application with the in-process anydoc parser engine linked in.
build-anydoc: anydoc-lib
	go build -tags anydoc -o $(BINARY_NAME) $(MAIN_PATH)

# Run the application
run: build
	./$(BINARY_NAME)

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Build Docker image
docker-build-app:
	@echo "获取版本信息..."
	@eval $$(./scripts/get_version.sh env); \
	./scripts/get_version.sh info; \
	docker build --platform $(PLATFORM) \
		--build-arg VERSION_ARG="$$VERSION" \
		--build-arg COMMIT_ID_ARG="$$COMMIT_ID" \
		--build-arg BUILD_TIME_ARG="$$BUILD_TIME" \
		--build-arg GO_VERSION_ARG="$$GO_VERSION" \
		--build-arg WITH_ANYDOC=$${WITH_ANYDOC:-1} \
		-f docker/Dockerfile.app -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Build docreader Docker image
docker-build-docreader:
	docker build --platform $(PLATFORM) -f docker/Dockerfile.docreader -t magicyuan876/yuheng-docreader:latest .

# Build frontend Docker image
docker-build-frontend:
	./scripts/build_frontend_dist.sh
	docker build --platform $(PLATFORM) -f frontend/Dockerfile -t magicyuan876/yuheng-ui:latest frontend/

# Build all Docker images
docker-build-all: docker-build-app docker-build-docreader docker-build-frontend

# Run Docker container (传统方式)
# Touch .env if missing — docker-compose.yml's `env_file: [.env]` is required
# for ${ENV} interpolation in builtin_models.yaml and would otherwise refuse
# to parse on fresh clones. `start-all` handles this via check_env_file; this
# direct path needs its own guard.
docker-run:
	@[ -f .env ] || ([ -f .env.example ] && cp .env.example .env || touch .env)
	docker-compose up

# 使用新脚本启动所有服务
start-all:
	./scripts/start_all.sh

# 使用新脚本仅启动Ollama服务
start-ollama:
	./scripts/start_all.sh --ollama

# 使用新脚本仅启动Docker容器
start-docker:
	./scripts/start_all.sh --docker

# 使用新脚本停止所有服务
stop-all:
	./scripts/start_all.sh --stop

# Stop Docker container (传统方式)
docker-stop:
	docker-compose down

# 从源码构建镜像相关命令
build-images:
	./scripts/build_images.sh

build-images-app:
	./scripts/build_images.sh --app

build-images-docreader:
	./scripts/build_images.sh --docreader

build-images-frontend:
	./scripts/build_images.sh --frontend

# 在线文档的协同编辑服务。没有它，在线文档依然可用（退化为独占编辑），
# 所以它不在 build-images 里，需要时单独构建。
build-images-collab:
	./scripts/build_images.sh --collab

clean-images:
	./scripts/build_images.sh --clean

# Restart Docker container (stop, start)
docker-restart:
	@[ -f .env ] || ([ -f .env.example ] && cp .env.example .env || touch .env)
	docker-compose stop -t 60
	docker-compose up

# Database migrations
migrate-up:
	./scripts/migrate.sh up

migrate-down:
	./scripts/migrate.sh down

migrate-version:
	./scripts/migrate.sh version

migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Error: migration name is required"; \
		echo "Usage: make migrate-create name=your_migration_name"; \
		exit 1; \
	fi
	./scripts/migrate.sh create $(name)

migrate-force:
	@if [ -z "$(version)" ]; then \
		echo "Error: version is required"; \
		echo "Usage: make migrate-force version=4"; \
		exit 1; \
	fi
	./scripts/migrate.sh force $(version)

migrate-goto:
	@if [ -z "$(version)" ]; then \
		echo "Error: version is required"; \
		echo "Usage: make migrate-goto version=3"; \
		exit 1; \
	fi
	./scripts/migrate.sh goto $(version)

# Generate API documentation (Swagger)
docs:
	@echo "生成 Swagger API 文档..."
	swag init -g $(MAIN_PATH)/main.go -d ./,./internal/docs/handler -o ./docs --parseDependency --parseInternal
	@echo "文档已生成到 ./docs 目录"
	@echo "启动服务后访问 http://localhost:8080/swagger/index.html 查看文档"

# Install swagger tool, at the swaggo/swag version go.mod pins: the generator
# and the swag runtime the server links must agree, and @latest regenerated
# docs/ differently from one machine to the next.
install-swagger:
	go install github.com/swaggo/swag/cmd/swag@$$(go list -m -f '{{.Version}}' github.com/swaggo/swag)

# Format code. gofumpt (a stricter gofmt), pinned to the version golangci-lint
# embeds, over the whole root module: see scripts/gofumpt-tree.sh.
fmt:
	./scripts/gofumpt-tree.sh write

# Fail if any Go file is not formatted; CI runs the same check.
fmt-check:
	./scripts/gofumpt-tree.sh check

# Lint code
lint:
	golangci-lint run

# Install dependencies
deps:
	go mod download

# Build for production
# GO_BUILD_TAGS adds optional build tags, e.g. GO_BUILD_TAGS=anydoc to link the
# in-process office document parser (run `make anydoc-lib` first).
build-prod:
	VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "$${VERSION:-unknown}"); \
	COMMIT_ID=$${COMMIT_ID:-unknown}; \
	CGO_ENABLED=1 \
	CGO_CFLAGS="-Wno-deprecated-declarations" \
	CGO_LDFLAGS="$$(if [ "$$(uname)" = 'Darwin' ]; then echo '-Wl,-no_warn_duplicate_libraries'; fi)" \
	BUILD_TIME=$${BUILD_TIME:-unknown}; \
	GO_VERSION=$${GO_VERSION:-unknown}; \
	LDFLAGS="-X 'github.com/magicyuan876/yuheng/internal/handler.Version=$$VERSION' -X 'github.com/magicyuan876/yuheng/internal/handler.CommitID=$$COMMIT_ID' -X 'github.com/magicyuan876/yuheng/internal/handler.BuildTime=$$BUILD_TIME' -X 'github.com/magicyuan876/yuheng/internal/handler.GoVersion=$$GO_VERSION'"; \
	go build -tags "$(GO_BUILD_TAGS)" -ldflags="-w -s $$LDFLAGS" -o $(BINARY_NAME) $(MAIN_PATH)

download_spatial:
	go run cmd/download/duckdb/duckdb.go

# The volumes clean-db deletes, by their key in docker-compose.yml. Docker
# creates each one as <project>_<key>, and the project name is not fixed: it is
# the checkout's directory name unless COMPOSE_PROJECT_NAME or a top-level
# `name:` says otherwise. Hard-coding "yuheng_<key>" therefore broke twice over —
# on any other checkout name, and when the redis key was written as redis_data
# while compose declares redis-data, so Redis silently survived every clean-db.
# The recipe asks compose for the project name, refuses a key compose does not
# declare (so a rename fails loudly instead of skipping), and finds the volume
# by the labels compose puts on it rather than by a name filter, which is a
# substring match.
CLEAN_DB_VOLUMES := postgres-data rustfs_data redis-data

clean-db:
	@project=$$(docker compose config 2>/dev/null | sed -n '1s/^name: //p'); \
	if [ -z "$$project" ]; then \
		echo "clean-db: 'docker compose config' failed; run it to see why (a missing .env is the usual cause)" >&2; \
		exit 1; \
	fi; \
	declared=" $$(docker compose --profile '*' config --volumes 2>/dev/null | tr '\n' ' ') "; \
	for key in $(CLEAN_DB_VOLUMES); do \
		case "$$declared" in *" $$key "*) ;; *) \
			echo "clean-db: docker-compose.yml declares no volume '$$key'; update CLEAN_DB_VOLUMES" >&2; \
			exit 1;; \
		esac; \
	done; \
	echo "Cleaning database volumes of compose project '$$project'..."; \
	for key in $(CLEAN_DB_VOLUMES); do \
		vol=$$(docker volume ls -q \
			--filter "label=com.docker.compose.project=$$project" \
			--filter "label=com.docker.compose.volume=$$key"); \
		if [ -n "$$vol" ]; then \
			docker volume rm $$vol || { echo "clean-db: stop the stack first (docker compose down)" >&2; exit 1; }; \
		else \
			echo "  $$key: no such volume, skipped"; \
		fi; \
	done

# Environment check
check-env:
	./scripts/start_all.sh --check

# List containers
list-containers:
	./scripts/start_all.sh --list

# Pull latest images
pull-images:
	./scripts/start_all.sh --pull

# Show current platform
show-platform:
	@echo "当前系统架构: $(shell uname -m)"
	@echo "Docker构建平台: $(PLATFORM)"

# Development mode commands
dev-start:
	./scripts/dev.sh start $(DEV_ARGS)

dev-stop:
	./scripts/dev.sh stop

dev-restart:
	./scripts/dev.sh restart

dev-logs:
	./scripts/dev.sh logs

dev-status:
	./scripts/dev.sh status

dev-app:
	./scripts/dev.sh app

dev-frontend:
	./scripts/dev.sh frontend


