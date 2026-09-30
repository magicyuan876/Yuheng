# Build stage
FROM golang:1.26-bookworm AS builder

WORKDIR /app

# 通过构建参数接收敏感信息
ARG GOPRIVATE_ARG
ARG GOPROXY_ARG
ARG GOSUMDB_ARG=off
ARG APK_MIRROR_ARG

# 设置Go环境变量
ENV GOPRIVATE=${GOPRIVATE_ARG}
ENV GOPROXY=${GOPROXY_ARG}
ENV GOSUMDB=${GOSUMDB_ARG}

# Install dependencies
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
        sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y git build-essential curl

# Copy go mod files. go.mod replace-points anydoc at ./third_party/anydoc-go,
# so that module's go.mod must exist before `go mod download`.
COPY go.mod go.sum ./
COPY third_party/anydoc-go/go.mod third_party/anydoc-go/go.mod
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# The migrate CLI behind scripts/migrate.sh, at the golang-migrate version the
# server itself links (go.mod), not @latest: the CLI and the server share the
# schema_migrations table and its dirty flag, and must agree on both.
RUN --mount=type=cache,target=/go/pkg/mod \
    go install -tags 'postgres' \
        "github.com/golang-migrate/migrate/v4/cmd/migrate@$(go list -m -f '{{.Version}}' github.com/golang-migrate/migrate/v4)"
COPY cmd/download cmd/download
# Extensions pre-downloaded into docker/duckdb-extensions (see the README
# there) are used first, then those kept in a build cache from an earlier build;
# only what is still missing is downloaded (INSTALL skips an extension already
# present). The download is slow or fails from some networks, and without the
# cache every change to go.mod, which invalidates this layer, would repeat it.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/duckdb-extensions \
    --mount=type=bind,source=docker/duckdb-extensions,target=/tmp/duckdb-seed \
    mkdir -p /root/.duckdb/extensions && \
    cp -a /root/.cache/duckdb-extensions/. /root/.duckdb/extensions/ && \
    cp -a /tmp/duckdb-seed/. /root/.duckdb/extensions/ && rm -f /root/.duckdb/extensions/README.md && \
    go run cmd/download/duckdb/duckdb.go && \
    cp -a /root/.duckdb/extensions/. /root/.cache/duckdb-extensions/
COPY . .

# Get version and commit info for build injection
ARG VERSION_ARG
ARG COMMIT_ID_ARG
ARG BUILD_TIME_ARG
ARG GO_VERSION_ARG

# Set build-time variables
ENV VERSION=${VERSION_ARG}
ENV COMMIT_ID=${COMMIT_ID_ARG}
ENV BUILD_TIME=${BUILD_TIME_ARG}
ENV GO_VERSION=${GO_VERSION_ARG}

# Link the anydoc parser engine (office docs converted in-process, no
# Python docreader). Default on so Hub / compose images ship a working
# engine; pass WITH_ANYDOC=0 to skip the Rust toolchain (~few minutes and
# ~1 GB of build-stage layers).
ARG WITH_ANYDOC=1
ENV RUSTUP_HOME=/usr/local/rustup CARGO_HOME=/usr/local/cargo
ENV PATH=/usr/local/cargo/bin:$PATH
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    if [ "$WITH_ANYDOC" = "1" ]; then \
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
            | sh -s -- -y --profile minimal --default-toolchain stable && \
        ./scripts/build-anydoc-lib.sh; \
    fi

# Build the application with version info
RUN --mount=type=cache,target=/go/pkg/mod \
    if [ "$WITH_ANYDOC" = "1" ]; then \
        make build-prod GO_BUILD_TAGS=anydoc; \
    else \
        make build-prod; \
    fi
RUN --mount=type=cache,target=/go/pkg/mod cp -r /go/pkg/mod/github.com/yanyiwu/ /app/yanyiwu/

# Final stage
FROM debian:12.12-slim

WORKDIR /app

ARG APK_MIRROR_ARG

# Create a non-root user first
RUN useradd -m -s /bin/bash appuser

# First, install ca-certificates without mirror to ensure HTTPS works
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Then switch to mirror if specified and install the runtime packages. The
# list is what the running container actually uses, nothing more:
#   tzdata             time zones for the server (TZ in compose)
#   curl               the compose healthcheck (curl -f /ready)
#   gosu               docker-entrypoint.sh drops root to appuser with it
#   postgresql-client  psql for an operator in the container; with an external
#                      database (Helm postgresql.enabled=false) it is the only
#                      client inside the cluster network
# Python, Node/npm, uvx and a compiler used to be here for the built-in agent's
# MCP client, the MySQL client for MySQL support, and ffmpeg: those features are
# gone (PostgreSQL is the only database, video is parsed by docreader) and
# nothing in the image calls any of them.
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
        sed -i "s@deb.debian.org@${APK_MIRROR_ARG}@g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        tzdata curl gosu postgresql-client && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create data directories and set permissions
RUN mkdir -p /data/files && \
    chown -R appuser:appuser /app /data/files

# Copy migrate tool from builder stage
COPY --from=builder /go/bin/migrate /usr/local/bin/
COPY --from=builder /app/yanyiwu/ /go/pkg/mod/github.com/yanyiwu/

# Copy the binary from the builder stage
COPY --from=builder /app/config ./config
COPY --from=builder /app/scripts ./scripts
# The server embeds its migrations (migrations/embed.go) and no longer reads this
# directory. It stays for `docker exec Yuheng-app ./scripts/migrate.sh ...`, which
# drives the migrate CLI (copied above) against the SQL files.
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/dataset/samples ./dataset/samples
COPY --from=builder /root/.duckdb /home/appuser/.duckdb
COPY --from=builder /app/Yuheng .

# Copy and make entrypoint script executable
COPY --from=builder /app/scripts/docker-entrypoint.sh ./scripts/docker-entrypoint.sh

# Make scripts executable
RUN chmod +x ./scripts/*.sh

# Expose ports
EXPOSE 8080


ENTRYPOINT ["./scripts/docker-entrypoint.sh"]
CMD ["./Yuheng"]
