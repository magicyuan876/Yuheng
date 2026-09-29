# Third-Party Notices

This file lists the third-party components Yuheng depends on, with the license
each one is distributed under. It complements — it does not replace — the
`LICENSE` file, which carries Tencent's original MIT text for the upstream
WeKnora code together with the third-party notices that upstream compiled.
Full license texts are under [`licenses/`](./licenses/).

How this list was produced, and what it does *not* cover, is documented in
[`licenses/NOTICE_AUDIT.md`](./licenses/NOTICE_AUDIT.md). Read that file before
cutting a release: it carries the blocking items.

---

## Summary

| Ecosystem | Components | Source of truth |
|---|---|---|
| Go | 302 modules | `go.mod`, `cli/go.mod`, `client/go.mod` |
| npm (frontend) | 569 packages | `frontend/package-lock.json` (resolved from `frontend/node_modules`) |
| npm (collab) | 35 packages | `collab/package-lock.json` (resolved from `collab/node_modules`) |
| Python (docreader) | 66 distributions | `docreader/uv.lock` (resolved from the built image) |
| Python (mcp-server) | 55 distributions | `mcp-server/uv.lock` (resolved from the built image) |

---

## Bundled assets

| Asset | Origin | License |
|---|---|---|
| `docs/images/*`, `frontend/src/assets/img/*` | Inherited from upstream WeKnora | MIT, as part of the upstream repository |
| `skills/preloaded/*` | Inherited from upstream WeKnora | MIT, as part of the upstream repository |
| `internal/types/opencc/TSPhrases.txt`, `TSCharacters.txt` | [OpenCC](https://github.com/BYVoid/OpenCC) (Open Chinese Convert), © BYVoid and contributors | Apache-2.0 — copied unmodified; license text in `internal/types/opencc/LICENSE`, provenance in the README beside it |
| `third_party/anydoc-go/` | github.com/firecrawl/anydoc | MIT — Copyright (c) 2026 Sideguide Technologies Inc.; see `third_party/anydoc-go/LICENSE` |

---

## Go modules

| License | Modules |
|---|---|
| MIT | 133 |
| Apache-2.0 | 107 |
| BSD-3-Clause | 44 |
| BSD-2-Clause | 9 |
| ISC | 1 |
| MPL-2.0 | 1 |
| UNRESOLVED (windows-only; not in the linux module cache) | 2 |
| Apache-2.0 (v0.67.0 is replaced by v0.59.0 in go.mod; the replacement is in the cache and scans Apache-2.0) | 1 |
| MIT (this repository) | 1 |
| MIT (vendored at third_party/anydoc-go; see its LICENSE) | 1 |
| UNRESOLVED (not in the linux module cache) | 1 |
| UNRESOLVED (terminal input; not in the linux module cache) | 1 |

<details>
<summary>Full list (366 modules)</summary>

| Module | Version | License |
|---|---|---|
| `cloud.google.com/go/auth` | v0.20.0 | Apache-2.0 |
| `cloud.google.com/go/auth/oauth2adapt` | v0.2.8 | Apache-2.0 |
| `cloud.google.com/go/compute/metadata` | v0.9.0 | Apache-2.0 |
| `codeberg.org/readeck/go-readability/v2` | v2.1.2 | MIT |
| `dario.cat/mergo` | v1.0.2 | BSD-3-Clause |
| `filippo.io/edwards25519` | v1.2.0 | BSD-3-Clause |
| `github.com/alicebob/miniredis/v2` | v2.38.0 | MIT |
| `github.com/andybalholm/brotli` | v1.2.0 | MIT |
| `github.com/andybalholm/cascadia` | v1.3.3 | BSD-2-Clause |
| `github.com/apache/arrow-go/v18` | v18.5.1 | Apache-2.0 |
| `github.com/atotto/clipboard` | v0.1.4 | BSD-3-Clause |
| `github.com/aws/aws-sdk-go-v2` | v1.41.7 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` | v1.7.10 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/config` | v1.32.17 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.19.16 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/feature/ec2/imds` | v1.18.23 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/internal/configsources` | v1.4.23 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/internal/endpoints/v2` | v2.7.23 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/internal/v4a` | v1.4.24 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding` | v1.13.9 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/checksum` | v1.9.15 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/presigned-url` | v1.13.23 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/s3shared` | v1.19.23 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.101.0 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/signin` | v1.0.11 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/sso` | v1.30.17 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/ssooidc` | v1.35.21 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/sts` | v1.42.1 | Apache-2.0 |
| `github.com/aws/smithy-go` | v1.25.1 | Apache-2.0 |
| `github.com/aymanbagabas/go-osc52/v2` | v2.0.1 | MIT |
| `github.com/Azure/go-ansiterm` | v0.0.0-20250102033503-faa5f7b0171c | MIT |
| `github.com/bahlo/generic-list-go` | v0.2.0 | BSD-3-Clause |
| `github.com/beorn7/perks` | v1.0.1 | MIT |
| `github.com/buger/jsonparser` | v1.1.2 | MIT |
| `github.com/bytedance/gopkg` | v0.1.3 | Apache-2.0 |
| `github.com/bytedance/sonic` | v1.15.0 | Apache-2.0 |
| `github.com/bytedance/sonic/loader` | v0.5.0 | Apache-2.0 |
| `github.com/catppuccin/go` | v0.3.0 | MIT |
| `github.com/cenkalti/backoff/v4` | v4.3.0 | MIT |
| `github.com/cenkalti/backoff/v5` | v5.0.3 | MIT |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT |
| `github.com/charmbracelet/bubbles` | v0.21.1-0.20250623103423-23b8fd6302d7 | MIT |
| `github.com/charmbracelet/bubbletea` | v1.3.10 | MIT |
| `github.com/charmbracelet/colorprofile` | v0.2.3-0.20250311203215-f60798e515dc | MIT |
| `github.com/charmbracelet/huh` | v1.0.0 | MIT |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | MIT |
| `github.com/charmbracelet/x/ansi` | v0.10.1 | MIT |
| `github.com/charmbracelet/x/cellbuf` | v0.0.13-0.20250311204145-2c3ea96c31dd | MIT |
| `github.com/charmbracelet/x/exp/strings` | v0.0.0-20240722160745-212f7b056ed0 | MIT |
| `github.com/charmbracelet/x/term` | v0.2.1 | MIT |
| `github.com/chromedp/cdproto` | v0.0.0-20260321001828-e3e3800016bc | MIT |
| `github.com/chromedp/chromedp` | v0.15.1 | MIT |
| `github.com/chromedp/sysutil` | v1.1.0 | MIT |
| `github.com/clbanning/mxj` | v1.8.4 | BSD-3-Clause |
| `github.com/clipperhouse/stringish` | v0.1.1 | MIT |
| `github.com/clipperhouse/uax29/v2` | v2.3.0 | MIT |
| `github.com/cloudwego/base64x` | v0.1.6 | Apache-2.0 |
| `github.com/containerd/errdefs` | v1.0.0 | Apache-2.0 |
| `github.com/containerd/errdefs/pkg` | v0.3.0 | Apache-2.0 |
| `github.com/containerd/log` | v0.1.0 | Apache-2.0 |
| `github.com/containerd/platforms` | v0.2.1 | Apache-2.0 |
| `github.com/coreos/go-semver` | v0.3.0 | Apache-2.0 |
| `github.com/coreos/go-systemd/v22` | v22.3.2 | Apache-2.0 |
| `github.com/cpuguy83/dockercfg` | v0.3.2 | MIT |
| `github.com/danieljoos/wincred` | v1.1.2 | UNRESOLVED (windows-only; not in the linux module cache) |
| `github.com/DATA-DOG/go-sqlmock` | v1.5.2 | BSD-2-Clause |
| `github.com/davecgh/go-spew` | v1.1.2-0.20180830191138-d8f796af33cc | ISC |
| `github.com/dgryski/go-rendezvous` | v0.0.0-20200823014737-9f7001d12a5f | MIT |
| `github.com/distribution/reference` | v0.6.0 | Apache-2.0 |
| `github.com/docker/go-connections` | v0.7.0 | Apache-2.0 |
| `github.com/docker/go-units` | v0.5.0 | Apache-2.0 |
| `github.com/duckdb/duckdb-go-bindings` | v0.10502.0 | MIT |
| `github.com/duckdb/duckdb-go-bindings/lib/darwin-amd64` | v0.10502.0 | MIT |
| `github.com/duckdb/duckdb-go-bindings/lib/darwin-arm64` | v0.10502.0 | MIT |
| `github.com/duckdb/duckdb-go-bindings/lib/linux-amd64` | v0.10502.0 | MIT |
| `github.com/duckdb/duckdb-go-bindings/lib/linux-arm64` | v0.10502.0 | MIT |
| `github.com/duckdb/duckdb-go-bindings/lib/windows-amd64` | v0.10502.0 | MIT |
| `github.com/duckdb/duckdb-go/v2` | v2.10502.0 | MIT |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT |
| `github.com/ebitengine/purego` | v0.10.0 | Apache-2.0 |
| `github.com/erikgeiser/coninput` | v0.0.0-20211004153227-1c3628e74d0f | UNRESOLVED (terminal input; not in the linux module cache) |
| `github.com/felixge/httpsnoop` | v1.1.0 | MIT |
| `github.com/firecrawl/anydoc/go` | v0.1.8 | MIT (vendored at third_party/anydoc-go; see its LICENSE) |
| `github.com/form3tech-oss/jwt-go` | v3.2.5+incompatible | MIT |
| `github.com/fsnotify/fsnotify` | v1.9.0 | BSD-3-Clause |
| `github.com/gabriel-vasile/mimetype` | v1.4.12 | MIT |
| `github.com/gin-contrib/cors` | v1.7.7 | MIT |
| `github.com/gin-contrib/sse` | v1.1.0 | MIT |
| `github.com/gin-gonic/gin` | v1.12.0 | MIT |
| `github.com/go-json-experiment/json` | v0.0.0-20260214004413-d219187c3433 | BSD-3-Clause |
| `github.com/go-logr/logr` | v1.4.3 | Apache-2.0 |
| `github.com/go-logr/stdr` | v1.2.2 | Apache-2.0 |
| `github.com/go-ole/go-ole` | v1.3.0 | MIT |
| `github.com/go-openapi/jsonpointer` | v0.22.4 | Apache-2.0 |
| `github.com/go-openapi/jsonreference` | v0.21.4 | Apache-2.0 |
| `github.com/go-openapi/spec` | v0.22.3 | Apache-2.0 |
| `github.com/go-openapi/swag` | v0.19.15 | Apache-2.0 |
| `github.com/go-openapi/swag/conv` | v0.26.0 | Apache-2.0 |
| `github.com/go-openapi/swag/jsonname` | v0.26.0 | Apache-2.0 |
| `github.com/go-openapi/swag/jsonutils` | v0.26.0 | Apache-2.0 |
| `github.com/go-openapi/swag/loading` | v0.26.0 | Apache-2.0 |
| `github.com/go-openapi/swag/stringutils` | v0.26.0 | Apache-2.0 |
| `github.com/go-openapi/swag/typeutils` | v0.26.0 | Apache-2.0 |
| `github.com/go-openapi/swag/yamlutils` | v0.26.0 | Apache-2.0 |
| `github.com/go-playground/locales` | v0.14.1 | MIT |
| `github.com/go-playground/universal-translator` | v0.18.1 | MIT |
| `github.com/go-playground/validator/v10` | v10.30.1 | MIT |
| `github.com/go-shiori/dom` | v0.0.0-20230515143342-73569d674e1c | MIT |
| `github.com/go-sql-driver/mysql` | v1.10.0 | MPL-2.0 |
| `github.com/go-viper/mapstructure/v2` | v2.5.0 | MIT |
| `github.com/gobwas/httphead` | v0.1.0 | MIT |
| `github.com/gobwas/pool` | v0.2.1 | MIT |
| `github.com/gobwas/ws` | v1.4.0 | MIT |
| `github.com/goccy/go-json` | v0.10.5 | MIT |
| `github.com/goccy/go-yaml` | v1.19.2 | MIT |
| `github.com/godbus/dbus/v5` | v5.0.4 | BSD-2-Clause |
| `github.com/gogo/protobuf` | v1.3.2 | BSD-3-Clause |
| `github.com/gogs/chardet` | v0.0.0-20211120154057-b7413eaefb8f | MIT |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | MIT |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 | MIT |
| `github.com/golang/protobuf` | v1.5.4 | BSD-3-Clause |
| `github.com/google/btree` | v1.0.0 | Apache-2.0 |
| `github.com/google/flatbuffers` | v25.12.19+incompatible | Apache-2.0 |
| `github.com/google/go-querystring` | v1.1.0 | BSD-3-Clause |
| `github.com/google/jsonschema-go` | v0.4.3 | MIT |
| `github.com/google/s2a-go` | v0.1.9 | Apache-2.0 |
| `github.com/google/uuid` | v1.6.0 | BSD-3-Clause |
| `github.com/googleapis/enterprise-certificate-proxy` | v0.3.15 | Apache-2.0 |
| `github.com/googleapis/gax-go/v2` | v2.22.0 | BSD-3-Clause |
| `github.com/grpc-ecosystem/go-grpc-prometheus` | v1.2.0 | Apache-2.0 |
| `github.com/grpc-ecosystem/grpc-gateway` | v1.16.0 | BSD-3-Clause |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | v2.28.0 | BSD-3-Clause |
| `github.com/hibiken/asynq` | v0.26.0 | MIT |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | UNRESOLVED (windows-only; not in the linux module cache) |
| `github.com/itchyny/gojq` | v0.12.19 | MIT |
| `github.com/itchyny/timefmt-go` | v0.1.8 | MIT |
| `github.com/itlightning/dateparse` | v0.2.1 | MIT |
| `github.com/jackc/pgpassfile` | v1.0.0 | MIT |
| `github.com/jackc/pgservicefile` | v0.0.0-20240606120523-5a60cdf6a761 | MIT |
| `github.com/jackc/pgx/v5` | v5.9.2 | MIT |
| `github.com/jackc/puddle/v2` | v2.2.2 | MIT |
| `github.com/jinzhu/inflection` | v1.0.0 | MIT |
| `github.com/jinzhu/now` | v1.1.5 | MIT |
| `github.com/JohannesKaufmann/dom` | v0.2.0 | MIT |
| `github.com/JohannesKaufmann/html-to-markdown/v2` | v2.5.1 | MIT |
| `github.com/json-iterator/go` | v1.1.13-0.20220915233716-71ac16282d12 | MIT |
| `github.com/klauspost/compress` | v1.18.6 | Apache-2.0 |
| `github.com/klauspost/cpuid/v2` | v2.3.0 | MIT |
| `github.com/kr/pretty` | v0.3.1 | MIT |
| `github.com/kr/text` | v0.2.0 | MIT |
| `github.com/KyleBanks/depth` | v1.2.1 | MIT |
| `github.com/leodido/go-urn` | v1.4.0 | MIT |
| `github.com/lib/pq` | v1.10.9 | MIT |
| `github.com/lucasb-eyer/go-colorful` | v1.4.0 | MIT |
| `github.com/lufia/plan9stats` | v0.0.0-20260330125221-c963978e514e | BSD-3-Clause |
| `github.com/magiconair/properties` | v1.8.10 | BSD-2-Clause |
| `github.com/magicyuan876/yuheng/client` | v0.0.0-00010101000000-000000000000 | MIT (this repository) |
| `github.com/mailru/easyjson` | v0.9.0 | MIT |
| `github.com/mattn/go-isatty` | v0.0.22 | MIT |
| `github.com/mattn/go-localereader` | v0.0.1 | UNRESOLVED (not in the linux module cache) |
| `github.com/mattn/go-runewidth` | v0.0.16 | MIT |
| `github.com/Microsoft/go-winio` | v0.6.2 | MIT |
| `github.com/mitchellh/hashstructure/v2` | v2.0.2 | MIT |
| `github.com/mitchellh/mapstructure` | v1.4.2 | MIT |
| `github.com/mmcdole/gofeed` | v1.3.0 | MIT |
| `github.com/mmcdole/goxpp` | v1.1.1-0.20240225020742-a0c311522b23 | MIT |
| `github.com/moby/docker-image-spec` | v1.3.1 | Apache-2.0 |
| `github.com/moby/go-archive` | v0.2.0 | Apache-2.0 |
| `github.com/moby/moby/api` | v1.54.2 | Apache-2.0 |
| `github.com/moby/moby/client` | v0.4.1 | Apache-2.0 |
| `github.com/moby/patternmatcher` | v0.6.1 | Apache-2.0 |
| `github.com/moby/sys/sequential` | v0.6.0 | Apache-2.0 |
| `github.com/moby/sys/user` | v0.4.0 | Apache-2.0 |
| `github.com/moby/sys/userns` | v0.1.0 | Apache-2.0 |
| `github.com/moby/term` | v0.5.2 | Apache-2.0 |
| `github.com/modelcontextprotocol/go-sdk` | v1.8.0 | Apache-2.0 |
| `github.com/modern-go/concurrent` | v0.0.0-20180306012644-bacd9c7ef1dd | Apache-2.0 |
| `github.com/modern-go/reflect2` | v1.0.2 | Apache-2.0 |
| `github.com/muesli/ansi` | v0.0.0-20230316100256-276c6243b2f6 | MIT |
| `github.com/muesli/cancelreader` | v0.2.2 | MIT |
| `github.com/muesli/termenv` | v0.16.0 | MIT |
| `github.com/neo4j/neo4j-go-driver/v6` | v6.0.0 | Apache-2.0 |
| `github.com/ollama/ollama` | v0.23.2 | MIT |
| `github.com/opencontainers/go-digest` | v1.0.0 | Apache-2.0 |
| `github.com/opencontainers/image-spec` | v1.1.1 | Apache-2.0 |
| `github.com/panjf2000/ants/v2` | v2.12.0 | MIT |
| `github.com/parquet-go/bitpack` | v1.0.0 | Apache-2.0 |
| `github.com/parquet-go/jsonlite` | v1.0.0 | MIT |
| `github.com/parquet-go/parquet-go` | v0.29.0 | Apache-2.0 |
| `github.com/pelletier/go-toml/v2` | v2.2.4 | MIT |
| `github.com/pganalyze/pg_query_go/v6` | v6.2.2 | BSD-3-Clause |
| `github.com/pgvector/pgvector-go` | v0.3.0 | MIT |
| `github.com/pierrec/lz4/v4` | v4.1.25 | BSD-3-Clause |
| `github.com/pkg/errors` | v0.9.1 | BSD-2-Clause |
| `github.com/pmezard/go-difflib` | v1.0.1-0.20181226105442-5d4384ee4fb2 | BSD-2-Clause |
| `github.com/power-devops/perfstat` | v0.0.0-20240221224432-82ca36839d55 | MIT |
| `github.com/prometheus/client_golang` | v1.11.0 | Apache-2.0 |
| `github.com/prometheus/client_model` | v0.2.0 | Apache-2.0 |
| `github.com/prometheus/common` | v0.30.0 | Apache-2.0 |
| `github.com/prometheus/procfs` | v0.7.3 | Apache-2.0 |
| `github.com/PuerkitoBio/goquery` | v1.12.0 | BSD-3-Clause |
| `github.com/quic-go/qpack` | v0.6.0 | MIT |
| `github.com/quic-go/quic-go` | v0.59.1 | MIT |
| `github.com/redis/go-redis/v9` | v9.14.1 | BSD-2-Clause |
| `github.com/richardlehane/mscfb` | v1.0.7 | Apache-2.0 |
| `github.com/richardlehane/msoleps` | v1.0.6 | Apache-2.0 |
| `github.com/rivo/uniseg` | v0.4.7 | MIT |
| `github.com/robfig/cron/v3` | v3.0.1 | MIT |
| `github.com/rogpeppe/go-internal` | v1.14.1 | BSD-3-Clause |
| `github.com/sagikazarmark/locafero` | v0.11.0 | MIT |
| `github.com/sashabaranov/go-openai` | v1.41.2 | Apache-2.0 |
| `github.com/segmentio/asm` | v1.1.3 | MIT |
| `github.com/segmentio/encoding` | v0.5.4 | MIT |
| `github.com/shirou/gopsutil/v4` | v4.26.4 | BSD-3-Clause |
| `github.com/sirupsen/logrus` | v1.9.4 | MIT |
| `github.com/sourcegraph/conc` | v0.3.1-0.20240121214520-5f936abd7ae8 | MIT |
| `github.com/spaolacci/murmur3` | v0.0.0-20180118202830-f09979ecbc72 | BSD-3-Clause |
| `github.com/spf13/afero` | v1.15.0 | Apache-2.0 |
| `github.com/spf13/cast` | v1.10.0 | MIT |
| `github.com/spf13/cobra` | v1.7.0 | Apache-2.0 |
| `github.com/spf13/pflag` | v1.0.10 | BSD-3-Clause |
| `github.com/spf13/viper` | v1.21.0 | MIT |
| `github.com/stretchr/objx` | v0.5.3 | MIT |
| `github.com/stretchr/testify` | v1.11.1 | MIT |
| `github.com/subosito/gotenv` | v1.6.0 | MIT |
| `github.com/swaggo/files` | v1.0.1 | MIT |
| `github.com/swaggo/gin-swagger` | v1.6.1 | MIT |
| `github.com/swaggo/swag` | v1.16.6 | MIT |
| `github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common` | v1.3.103 | Apache-2.0 |
| `github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/lkeap` | v1.3.103 | Apache-2.0 |
| `github.com/testcontainers/testcontainers-go` | v0.42.0 | MIT |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | v0.42.0 | MIT |
| `github.com/tidwall/gjson` | v1.14.2 | MIT |
| `github.com/tidwall/match` | v1.1.1 | MIT |
| `github.com/tidwall/pretty` | v1.2.0 | MIT |
| `github.com/tiendc/go-deepcopy` | v1.7.2 | MIT |
| `github.com/tklauser/go-sysconf` | v0.3.16 | BSD-3-Clause |
| `github.com/tklauser/numcpus` | v0.11.0 | Apache-2.0 |
| `github.com/twitchyliquid64/golang-asm` | v0.15.1 | BSD-3-Clause |
| `github.com/twpayne/go-geom` | v1.6.1 | BSD-2-Clause |
| `github.com/ugorji/go/codec` | v1.3.1 | MIT |
| `github.com/volcengine/vikingdb-go-sdk` | v0.0.11 | Apache-2.0 |
| `github.com/volcengine/volc-sdk-golang` | v1.0.252 | Apache-2.0 |
| `github.com/wk8/go-ordered-map/v2` | v2.1.8 | Apache-2.0 |
| `github.com/x448/float16` | v0.8.4 | MIT |
| `github.com/xo/terminfo` | v0.0.0-20220910002029-abceb7e1c41e | MIT |
| `github.com/xuri/efp` | v0.0.1 | BSD-3-Clause |
| `github.com/xuri/excelize/v2` | v2.11.0 | BSD-3-Clause |
| `github.com/xuri/nfp` | v0.0.2-0.20250530014748-2ddeb826f9a9 | BSD-3-Clause |
| `github.com/yanyiwu/gojieba` | v1.4.7 | MIT |
| `github.com/yosida95/uritemplate/v3` | v3.0.2 | BSD-3-Clause |
| `github.com/yuin/goldmark` | v1.8.2 | MIT |
| `github.com/yuin/gopher-lua` | v1.1.1 | MIT |
| `github.com/yusufpapurcu/wmi` | v1.2.4 | MIT |
| `github.com/zalando/go-keyring` | v0.2.8 | MIT |
| `github.com/zeebo/xxh3` | v1.1.0 | BSD-2-Clause |
| `go.etcd.io/etcd/api/v3` | v3.5.0 | Apache-2.0 |
| `go.etcd.io/etcd/client/pkg/v3` | v3.5.0 | Apache-2.0 |
| `go.etcd.io/etcd/client/v2` | v2.305.0 | Apache-2.0 |
| `go.etcd.io/etcd/client/v3` | v3.5.0 | Apache-2.0 |
| `go.mongodb.org/mongo-driver/v2` | v2.5.0 | Apache-2.0 |
| `go.opentelemetry.io/auto/sdk` | v1.2.1 | Apache-2.0 |
| `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | v0.67.0 | Apache-2.0 (v0.67.0 is replaced by v0.59.0 in go.mod; the replacement is in the cache and scans Apache-2.0) |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | v0.69.0 | Apache-2.0 |
| `go.opentelemetry.io/otel` | v1.44.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.43.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | v1.43.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/metric` | v1.44.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/sdk` | v1.44.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/trace` | v1.44.0 | Apache-2.0 |
| `go.opentelemetry.io/proto/otlp` | v1.10.0 | Apache-2.0 |
| `go.uber.org/atomic` | v1.9.0 | MIT |
| `go.uber.org/dig` | v1.19.0 | MIT |
| `go.uber.org/multierr` | v1.7.0 | MIT |
| `go.uber.org/zap` | v1.19.1 | MIT |
| `go.yaml.in/yaml/v3` | v3.0.4 | MIT |
| `golang.org/x/arch` | v0.23.0 | BSD-3-Clause |
| `golang.org/x/crypto` | v0.55.0 | BSD-3-Clause |
| `golang.org/x/exp` | v0.0.0-20260112195511-716be5621a96 | BSD-3-Clause |
| `golang.org/x/mod` | v0.41.0 | BSD-3-Clause |
| `golang.org/x/net` | v0.58.0 | BSD-3-Clause |
| `golang.org/x/oauth2` | v0.36.0 | BSD-3-Clause |
| `golang.org/x/sync` | v0.23.0 | BSD-3-Clause |
| `golang.org/x/sys` | v0.47.0 | BSD-3-Clause |
| `golang.org/x/telemetry` | v0.0.0-20260811182544-a038080d80e5 | BSD-3-Clause |
| `golang.org/x/text` | v0.42.0 | BSD-3-Clause |
| `golang.org/x/time` | v0.15.0 | BSD-3-Clause |
| `golang.org/x/tools` | v0.49.0 | BSD-3-Clause |
| `golang.org/x/xerrors` | v0.0.0-20240903120638-7835f813f4da | BSD-3-Clause |
| `google.golang.org/api` | v0.278.0 | BSD-3-Clause |
| `google.golang.org/genproto` | v0.0.0-20260319201613-d00831a3d3e7 | Apache-2.0 |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260706201446-f0a921348800 | Apache-2.0 |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260706201446-f0a921348800 | Apache-2.0 |
| `google.golang.org/grpc` | v1.84.0 | Apache-2.0 |
| `google.golang.org/protobuf` | v1.36.11 | BSD-3-Clause |
| `gopkg.in/inf.v0` | v0.9.1 | BSD-3-Clause |
| `gopkg.in/natefinch/lumberjack.v2` | v2.2.1 | MIT |
| `gopkg.in/yaml.v2` | v2.4.0 | Apache-2.0 |
| `gopkg.in/yaml.v3` | v3.0.1 | MIT |
| `gorm.io/driver/postgres` | v1.6.0 | MIT |
| `gorm.io/gorm` | v1.31.1 | MIT |
| `sigs.k8s.io/yaml` | v1.3.0 | Apache-2.0 |

</details>

---

## npm packages (frontend)

| License | Packages |
|---|---|
| MIT | 474 |
| ISC | 50 |
| Apache-2.0 | 17 |
| BSD-3-Clause | 12 |
| BSD-2-Clause | 7 |
| (MIT AND Zlib) | 2 |
| CC-BY-4.0 | 1 |
| (MPL-2.0 OR Apache-2.0) | 1 |
| CC0-1.0 | 1 |
| (MIT OR GPL-3.0-or-later) | 1 |
| NOT-DECLARED | 1 |
| Unlicense | 1 |
| 0BSD | 1 |

Dual- and multi-licensed packages, and the licence Yuheng elects:

| Package | Declared | Elected |
|---|---|---|
| `dompurify` | MPL-2.0 OR Apache-2.0 | **Apache-2.0** — avoids the MPL file-level obligations |
| `jszip` | MIT OR GPL-3.0-or-later | **MIT** |
| `pako` | MIT AND Zlib | both apply; both are permissive |
| `caniuse-lite` | CC-BY-4.0 | attribution given here; build-time data, not shipped in the bundle |
| `khroma` | not declared in `package.json` | **MIT** — stated in its bundled `license` file and README |
| `fractional-indexing` | CC0-1.0 | a public-domain dedication; no obligations to meet |
| `@excalidraw/excalidraw` and its React runtime | MIT | loaded only when somebody opens a drawing for editing, in its own chunk |

<details>
<summary>Full list (569 packages)</summary>

| Package | Version | License |
|---|---|---|
| `@antfu/install-pkg` | 1.1.0 | MIT |
| `@babel/code-frame` | 7.29.7 | MIT |
| `@babel/compat-data` | 7.29.7 | MIT |
| `@babel/core` | 7.29.7 | MIT |
| `@babel/generator` | 7.29.7 | MIT |
| `@babel/helper-annotate-as-pure` | 7.27.3 | MIT |
| `@babel/helper-compilation-targets` | 7.29.7 | MIT |
| `@babel/helper-create-class-features-plugin` | 7.29.3 | MIT |
| `@babel/helper-globals` | 7.29.7 | MIT |
| `@babel/helper-member-expression-to-functions` | 7.28.5 | MIT |
| `@babel/helper-module-imports` | 7.29.7 | MIT |
| `@babel/helper-module-transforms` | 7.29.7 | MIT |
| `@babel/helper-optimise-call-expression` | 7.27.1 | MIT |
| `@babel/helper-plugin-utils` | 7.28.6 | MIT |
| `@babel/helper-replace-supers` | 7.28.6 | MIT |
| `@babel/helper-skip-transparent-expression-wrappers` | 7.27.1 | MIT |
| `@babel/helper-string-parser` | 7.29.7 | MIT |
| `@babel/helper-validator-identifier` | 7.29.7 | MIT |
| `@babel/helper-validator-option` | 7.29.7 | MIT |
| `@babel/helpers` | 7.29.7 | MIT |
| `@babel/parser` | 7.29.7 | MIT |
| `@babel/plugin-syntax-jsx` | 7.28.6 | MIT |
| `@babel/plugin-syntax-typescript` | 7.28.6 | MIT |
| `@babel/plugin-transform-typescript` | 7.28.6 | MIT |
| `@babel/runtime` | 7.27.6 | MIT |
| `@babel/template` | 7.29.7 | MIT |
| `@babel/traverse` | 7.29.7 | MIT |
| `@babel/types` | 7.29.7 | MIT |
| `@braintree/sanitize-url` | 6.0.2 | MIT |
| `@braintree/sanitize-url` | 7.1.2 | MIT |
| `@chevrotain/cst-dts-gen` | 11.0.3 | Apache-2.0 |
| `@chevrotain/gast` | 11.0.3 | Apache-2.0 |
| `@chevrotain/regexp-to-ast` | 11.0.3 | Apache-2.0 |
| `@chevrotain/types` | 11.0.3 | Apache-2.0 |
| `@chevrotain/types` | 11.1.2 | Apache-2.0 |
| `@chevrotain/utils` | 11.0.3 | Apache-2.0 |
| `@esbuild/win32-x64` | 0.25.6 | MIT |
| `@excalidraw/excalidraw` | 0.18.1 | MIT |
| `@excalidraw/laser-pointer` | 1.3.1 | MIT |
| `@excalidraw/markdown-to-text` | 0.1.2 | MIT |
| `@excalidraw/mermaid-to-excalidraw` | 2.2.2 | MIT |
| `@excalidraw/random-username` | 1.1.0 | MIT |
| `@floating-ui/core` | 1.8.0 | MIT |
| `@floating-ui/dom` | 1.8.0 | MIT |
| `@floating-ui/react-dom` | 2.1.9 | MIT |
| `@floating-ui/utils` | 0.2.12 | MIT |
| `@hocuspocus/common` | 4.7.0 | MIT |
| `@hocuspocus/provider` | 4.7.0 | MIT |
| `@iconify/types` | 2.0.0 | MIT |
| `@iconify/utils` | 3.1.0 | MIT |
| `@intlify/core-base` | 11.4.2 | MIT |
| `@intlify/devtools-types` | 11.4.2 | MIT |
| `@intlify/message-compiler` | 11.4.2 | MIT |
| `@intlify/shared` | 11.4.2 | MIT |
| `@jridgewell/gen-mapping` | 0.3.12 | MIT |
| `@jridgewell/remapping` | 2.3.5 | MIT |
| `@jridgewell/resolve-uri` | 3.1.2 | MIT |
| `@jridgewell/source-map` | 0.3.10 | MIT |
| `@jridgewell/sourcemap-codec` | 1.5.5 | MIT |
| `@jridgewell/trace-mapping` | 0.3.29 | MIT |
| `@lifeomic/attempt` | 3.1.0 | MIT |
| `@mermaid-js/parser` | 0.6.3 | MIT |
| `@mermaid-js/parser` | 1.1.1 | MIT |
| `@microsoft/fetch-event-source` | 2.0.1 | MIT |
| `@pagefind/windows-x64` | 1.5.2 | MIT |
| `@popperjs/core` | 2.11.8 | MIT |
| `@radix-ui/primitive` | 1.0.0 | MIT |
| `@radix-ui/primitive` | 1.1.1 | MIT |
| `@radix-ui/react-arrow` | 1.1.2 | MIT |
| `@radix-ui/react-collection` | 1.0.1 | MIT |
| `@radix-ui/react-compose-refs` | 1.0.0 | MIT |
| `@radix-ui/react-compose-refs` | 1.1.1 | MIT |
| `@radix-ui/react-context` | 1.0.0 | MIT |
| `@radix-ui/react-context` | 1.1.1 | MIT |
| `@radix-ui/react-direction` | 1.0.0 | MIT |
| `@radix-ui/react-dismissable-layer` | 1.1.5 | MIT |
| `@radix-ui/react-focus-guards` | 1.1.1 | MIT |
| `@radix-ui/react-focus-scope` | 1.1.2 | MIT |
| `@radix-ui/react-id` | 1.0.0 | MIT |
| `@radix-ui/react-id` | 1.1.0 | MIT |
| `@radix-ui/react-popover` | 1.1.6 | MIT |
| `@radix-ui/react-popper` | 1.2.2 | MIT |
| `@radix-ui/react-portal` | 1.1.4 | MIT |
| `@radix-ui/react-presence` | 1.0.0 | MIT |
| `@radix-ui/react-presence` | 1.1.2 | MIT |
| `@radix-ui/react-primitive` | 1.0.1 | MIT |
| `@radix-ui/react-primitive` | 2.0.2 | MIT |
| `@radix-ui/react-roving-focus` | 1.0.2 | MIT |
| `@radix-ui/react-slot` | 1.0.1 | MIT |
| `@radix-ui/react-slot` | 1.1.2 | MIT |
| `@radix-ui/react-tabs` | 1.0.2 | MIT |
| `@radix-ui/react-use-callback-ref` | 1.0.0 | MIT |
| `@radix-ui/react-use-callback-ref` | 1.1.0 | MIT |
| `@radix-ui/react-use-controllable-state` | 1.0.0 | MIT |
| `@radix-ui/react-use-controllable-state` | 1.1.0 | MIT |
| `@radix-ui/react-use-escape-keydown` | 1.1.0 | MIT |
| `@radix-ui/react-use-layout-effect` | 1.0.0 | MIT |
| `@radix-ui/react-use-layout-effect` | 1.1.0 | MIT |
| `@radix-ui/react-use-rect` | 1.1.0 | MIT |
| `@radix-ui/react-use-size` | 1.1.0 | MIT |
| `@radix-ui/rect` | 1.1.0 | MIT |
| `@rolldown/pluginutils` | 1.0.0-rc.13 | MIT |
| `@rollup/rollup-win32-x64-gnu` | 4.59.0 | MIT |
| `@rollup/rollup-win32-x64-msvc` | 4.59.0 | MIT |
| `@tiptap/core` | 3.31.3 | MIT |
| `@tiptap/extension-blockquote` | 3.31.3 | MIT |
| `@tiptap/extension-bold` | 3.31.3 | MIT |
| `@tiptap/extension-bubble-menu` | 3.31.3 | MIT |
| `@tiptap/extension-code` | 3.31.3 | MIT |
| `@tiptap/extension-code-block` | 3.31.3 | MIT |
| `@tiptap/extension-code-block-lowlight` | 3.31.3 | MIT |
| `@tiptap/extension-collaboration` | 3.31.3 | MIT |
| `@tiptap/extension-collaboration-caret` | 3.31.3 | MIT |
| `@tiptap/extension-details` | 3.31.3 | MIT |
| `@tiptap/extension-document` | 3.31.3 | MIT |
| `@tiptap/extension-dropcursor` | 3.31.3 | MIT |
| `@tiptap/extension-floating-menu` | 3.31.3 | MIT |
| `@tiptap/extension-gapcursor` | 3.31.3 | MIT |
| `@tiptap/extension-hard-break` | 3.31.3 | MIT |
| `@tiptap/extension-heading` | 3.31.3 | MIT |
| `@tiptap/extension-highlight` | 3.31.3 | MIT |
| `@tiptap/extension-horizontal-rule` | 3.31.3 | MIT |
| `@tiptap/extension-italic` | 3.31.3 | MIT |
| `@tiptap/extension-link` | 3.31.3 | MIT |
| `@tiptap/extension-list` | 3.31.3 | MIT |
| `@tiptap/extension-paragraph` | 3.31.3 | MIT |
| `@tiptap/extension-strike` | 3.31.3 | MIT |
| `@tiptap/extension-subscript` | 3.31.3 | MIT |
| `@tiptap/extension-superscript` | 3.31.3 | MIT |
| `@tiptap/extension-table` | 3.31.3 | MIT |
| `@tiptap/extension-text` | 3.31.3 | MIT |
| `@tiptap/extension-text-style` | 3.31.3 | MIT |
| `@tiptap/extension-underline` | 3.31.3 | MIT |
| `@tiptap/extensions` | 3.31.3 | MIT |
| `@tiptap/pm` | 3.31.3 | MIT |
| `@tiptap/vue-3` | 3.31.3 | MIT |
| `@tiptap/y-tiptap` | 3.0.9 | MIT |
| `@tsconfig/node22` | 22.0.5 | MIT |
| `@types/d3` | 7.4.3 | MIT |
| `@types/d3-array` | 3.2.2 | MIT |
| `@types/d3-axis` | 3.0.6 | MIT |
| `@types/d3-brush` | 3.0.6 | MIT |
| `@types/d3-chord` | 3.0.6 | MIT |
| `@types/d3-color` | 3.1.3 | MIT |
| `@types/d3-contour` | 3.0.6 | MIT |
| `@types/d3-delaunay` | 6.0.4 | MIT |
| `@types/d3-dispatch` | 3.0.7 | MIT |
| `@types/d3-drag` | 3.0.7 | MIT |
| `@types/d3-dsv` | 3.0.7 | MIT |
| `@types/d3-ease` | 3.0.2 | MIT |
| `@types/d3-fetch` | 3.0.7 | MIT |
| `@types/d3-force` | 3.0.10 | MIT |
| `@types/d3-format` | 3.0.4 | MIT |
| `@types/d3-geo` | 3.1.0 | MIT |
| `@types/d3-hierarchy` | 3.1.7 | MIT |
| `@types/d3-interpolate` | 3.0.4 | MIT |
| `@types/d3-path` | 3.1.1 | MIT |
| `@types/d3-polygon` | 3.0.2 | MIT |
| `@types/d3-quadtree` | 3.0.6 | MIT |
| `@types/d3-random` | 3.0.3 | MIT |
| `@types/d3-scale` | 4.0.9 | MIT |
| `@types/d3-scale-chromatic` | 3.1.0 | MIT |
| `@types/d3-selection` | 3.0.11 | MIT |
| `@types/d3-shape` | 3.1.8 | MIT |
| `@types/d3-time` | 3.0.4 | MIT |
| `@types/d3-time-format` | 4.0.3 | MIT |
| `@types/d3-timer` | 3.0.2 | MIT |
| `@types/d3-transition` | 3.0.9 | MIT |
| `@types/d3-zoom` | 3.0.8 | MIT |
| `@types/dompurify` | 3.2.0 | MIT |
| `@types/eslint` | 9.6.1 | MIT |
| `@types/eslint-scope` | 3.7.7 | MIT |
| `@types/estree` | 1.0.8 | MIT |
| `@types/geojson` | 7946.0.16 | MIT |
| `@types/hast` | 3.0.5 | MIT |
| `@types/json-schema` | 7.0.15 | MIT |
| `@types/lodash` | 4.17.20 | MIT |
| `@types/lodash-es` | 4.17.12 | MIT |
| `@types/node` | 22.16.3 | MIT |
| `@types/papaparse` | 5.5.2 | MIT |
| `@types/react` | 19.3.0 | MIT |
| `@types/react-dom` | 19.3.0 | MIT |
| `@types/sortablejs` | 1.15.8 | MIT |
| `@types/tinycolor2` | 1.4.6 | MIT |
| `@types/trusted-types` | 2.0.7 | MIT |
| `@types/unist` | 3.0.3 | MIT |
| `@types/validator` | 13.15.2 | MIT |
| `@upsetjs/venn.js` | 2.0.0 | MIT |
| `@vitejs/plugin-vue` | 6.0.6 | MIT |
| `@vitejs/plugin-vue-jsx` | 5.1.5 | MIT |
| `@volar/language-core` | 2.4.28 | MIT |
| `@volar/source-map` | 2.4.28 | MIT |
| `@volar/typescript` | 2.4.28 | MIT |
| `@vue-office/pptx` | 1.0.1 | MIT |
| `@vue/babel-helper-vue-transform-on` | 2.0.1 | MIT |
| `@vue/babel-plugin-jsx` | 2.0.1 | MIT |
| `@vue/babel-plugin-resolve-type` | 2.0.1 | MIT |
| `@vue/compiler-core` | 3.5.34 | MIT |
| `@vue/compiler-dom` | 3.5.34 | MIT |
| `@vue/compiler-sfc` | 3.5.34 | MIT |
| `@vue/compiler-ssr` | 3.5.34 | MIT |
| `@vue/devtools-api` | 6.6.4 | MIT |
| `@vue/devtools-api` | 7.7.7 | MIT |
| `@vue/devtools-kit` | 7.7.7 | MIT |
| `@vue/devtools-shared` | 7.7.7 | MIT |
| `@vue/language-core` | 3.2.8 | MIT |
| `@vue/reactivity` | 3.5.34 | MIT |
| `@vue/runtime-core` | 3.5.34 | MIT |
| `@vue/runtime-dom` | 3.5.34 | MIT |
| `@vue/server-renderer` | 3.5.34 | MIT |
| `@vue/shared` | 3.5.34 | MIT |
| `@vue/tsconfig` | 0.9.1 | MIT |
| `@webassemblyjs/ast` | 1.14.1 | MIT |
| `@webassemblyjs/floating-point-hex-parser` | 1.13.2 | MIT |
| `@webassemblyjs/helper-api-error` | 1.13.2 | MIT |
| `@webassemblyjs/helper-buffer` | 1.14.1 | MIT |
| `@webassemblyjs/helper-numbers` | 1.13.2 | MIT |
| `@webassemblyjs/helper-wasm-bytecode` | 1.13.2 | MIT |
| `@webassemblyjs/helper-wasm-section` | 1.14.1 | MIT |
| `@webassemblyjs/ieee754` | 1.13.2 | MIT |
| `@webassemblyjs/leb128` | 1.13.2 | Apache-2.0 |
| `@webassemblyjs/utf8` | 1.13.2 | MIT |
| `@webassemblyjs/wasm-edit` | 1.14.1 | MIT |
| `@webassemblyjs/wasm-gen` | 1.14.1 | MIT |
| `@webassemblyjs/wasm-opt` | 1.14.1 | MIT |
| `@webassemblyjs/wasm-parser` | 1.14.1 | MIT |
| `@webassemblyjs/wast-printer` | 1.14.1 | MIT |
| `@xtuc/ieee754` | 1.2.0 | BSD-3-Clause |
| `@xtuc/long` | 4.2.2 | Apache-2.0 |
| `acorn` | 8.16.0 | MIT |
| `acorn-import-phases` | 1.0.4 | MIT |
| `ajv` | 8.18.0 | MIT |
| `ajv-formats` | 2.1.1 | MIT |
| `ajv-keywords` | 5.1.0 | MIT |
| `alien-signals` | 3.1.2 | MIT |
| `ansi-styles` | 6.2.1 | MIT |
| `anymatch` | 3.1.3 | ISC |
| `aria-hidden` | 1.2.6 | MIT |
| `asynckit` | 0.4.0 | MIT |
| `axios` | 1.16.0 | MIT |
| `baseline-browser-mapping` | 2.10.0 | Apache-2.0 |
| `binary-extensions` | 2.3.0 | MIT |
| `birpc` | 2.5.0 | MIT |
| `braces` | 3.0.3 | MIT |
| `browser-fs-access` | 0.29.1 | Apache-2.0 |
| `browserslist` | 4.28.1 | MIT |
| `buffer-from` | 1.1.2 | MIT |
| `call-bind-apply-helpers` | 1.0.2 | MIT |
| `caniuse-lite` | 1.0.30001774 | CC-BY-4.0 |
| `canvas-roundrect-polyfill` | 0.0.1 | MIT |
| `chevrotain` | 11.0.3 | Apache-2.0 |
| `chevrotain-allstar` | 0.3.1 | MIT |
| `chokidar` | 3.6.0 | MIT |
| `chrome-trace-event` | 1.0.4 | MIT |
| `clsx` | 1.1.1 | MIT |
| `combined-stream` | 1.0.8 | MIT |
| `commander` | 2.20.3 | MIT |
| `commander` | 7.2.0 | MIT |
| `commander` | 8.3.0 | MIT |
| `confbox` | 0.1.8 | MIT |
| `convert-source-map` | 2.0.0 | MIT |
| `copy-anything` | 3.0.5 | MIT |
| `core-util-is` | 1.0.3 | MIT |
| `cose-base` | 1.0.3 | MIT |
| `cose-base` | 2.2.0 | MIT |
| `crc-32` | 0.3.0 | Apache-2.0 |
| `cross-env` | 7.0.3 | MIT |
| `cross-spawn` | 7.0.6 | MIT |
| `csstype` | 3.2.3 | MIT |
| `cytoscape` | 3.33.2 | MIT |
| `cytoscape-cose-bilkent` | 4.1.0 | MIT |
| `cytoscape-fcose` | 2.2.0 | MIT |
| `d3` | 7.9.0 | ISC |
| `d3-array` | 2.12.1 | BSD-3-Clause |
| `d3-array` | 3.2.4 | ISC |
| `d3-axis` | 3.0.0 | ISC |
| `d3-brush` | 3.0.0 | ISC |
| `d3-chord` | 3.0.1 | ISC |
| `d3-color` | 3.1.0 | ISC |
| `d3-contour` | 4.0.2 | ISC |
| `d3-delaunay` | 6.0.4 | ISC |
| `d3-dispatch` | 3.0.1 | ISC |
| `d3-drag` | 3.0.0 | ISC |
| `d3-dsv` | 3.0.1 | ISC |
| `d3-ease` | 3.0.1 | BSD-3-Clause |
| `d3-fetch` | 3.0.1 | ISC |
| `d3-force` | 3.0.0 | ISC |
| `d3-format` | 3.1.2 | ISC |
| `d3-geo` | 3.1.1 | ISC |
| `d3-hierarchy` | 3.1.2 | ISC |
| `d3-interpolate` | 3.0.1 | ISC |
| `d3-path` | 1.0.9 | BSD-3-Clause |
| `d3-path` | 3.1.0 | ISC |
| `d3-polygon` | 3.0.1 | ISC |
| `d3-quadtree` | 3.0.1 | ISC |
| `d3-random` | 3.0.1 | ISC |
| `d3-sankey` | 0.12.3 | BSD-3-Clause |
| `d3-scale` | 4.0.2 | ISC |
| `d3-scale-chromatic` | 3.1.0 | ISC |
| `d3-selection` | 3.0.0 | ISC |
| `d3-shape` | 1.3.7 | BSD-3-Clause |
| `d3-shape` | 3.2.0 | ISC |
| `d3-time` | 3.1.0 | ISC |
| `d3-time-format` | 4.1.0 | ISC |
| `d3-timer` | 3.0.1 | ISC |
| `d3-transition` | 3.0.1 | ISC |
| `d3-zoom` | 3.0.0 | ISC |
| `dagre-d3-es` | 7.0.14 | MIT |
| `dayjs` | 1.11.19 | MIT |
| `debug` | 4.4.3 | MIT |
| `delaunator` | 5.1.0 | ISC |
| `delayed-stream` | 1.0.0 | MIT |
| `dequal` | 2.0.3 | MIT |
| `detect-node-es` | 1.1.0 | MIT |
| `devlop` | 1.1.0 | MIT |
| `docx-preview` | 0.3.7 | Apache-2.0 |
| `dompurify` | 3.4.11 | (MPL-2.0 OR Apache-2.0) |
| `dunder-proto` | 1.0.1 | MIT |
| `electron-to-chromium` | 1.5.302 | ISC |
| `enhanced-resolve` | 5.21.3 | MIT |
| `entities` | 7.0.1 | BSD-2-Clause |
| `errno` | 0.1.8 | MIT |
| `es-define-property` | 1.0.1 | MIT |
| `es-errors` | 1.3.0 | MIT |
| `es-module-lexer` | 2.0.0 | MIT |
| `es-object-atoms` | 1.1.1 | MIT |
| `es-set-tostringtag` | 2.1.0 | MIT |
| `es-toolkit` | 1.46.1 | MIT |
| `es6-promise-pool` | 2.5.0 | MIT |
| `esbuild` | 0.25.6 | MIT |
| `escalade` | 3.2.0 | MIT |
| `eslint-scope` | 5.1.1 | BSD-2-Clause |
| `esrecurse` | 4.3.0 | BSD-2-Clause |
| `estraverse` | 4.3.0 | BSD-2-Clause |
| `estraverse` | 5.3.0 | BSD-2-Clause |
| `estree-walker` | 2.0.2 | MIT |
| `events` | 3.3.0 | MIT |
| `fast-deep-equal` | 3.1.3 | MIT |
| `fast-uri` | 3.1.3 | BSD-3-Clause |
| `fdir` | 6.5.0 | MIT |
| `fill-range` | 7.1.1 | MIT |
| `follow-redirects` | 1.16.0 | MIT |
| `form-data` | 4.0.6 | MIT |
| `fractional-indexing` | 3.2.0 | CC0-1.0 |
| `function-bind` | 1.1.2 | MIT |
| `fuzzy` | 0.1.3 | MIT |
| `gensync` | 1.0.0-beta.2 | MIT |
| `get-intrinsic` | 1.3.0 | MIT |
| `get-nonce` | 1.0.1 | MIT |
| `get-proto` | 1.0.1 | MIT |
| `glob-parent` | 5.1.2 | ISC |
| `glob-to-regexp` | 0.4.1 | BSD-2-Clause |
| `glur` | 1.1.2 | MIT |
| `gopd` | 1.2.0 | MIT |
| `graceful-fs` | 4.2.11 | ISC |
| `hachure-fill` | 0.5.2 | MIT |
| `has-flag` | 4.0.0 | MIT |
| `has-symbols` | 1.1.0 | MIT |
| `has-tostringtag` | 1.0.2 | MIT |
| `hasown` | 2.0.4 | MIT |
| `highlight.js` | 11.11.1 | BSD-3-Clause |
| `hookable` | 5.5.3 | MIT |
| `iconv-lite` | 0.6.3 | MIT |
| `image-blob-reduce` | 3.0.1 | MIT |
| `image-size` | 0.5.5 | MIT |
| `immediate` | 3.0.6 | MIT |
| `immutable` | 4.3.9 | MIT |
| `inherits` | 2.0.4 | ISC |
| `internmap` | 1.0.1 | ISC |
| `internmap` | 2.0.3 | ISC |
| `is-binary-path` | 2.1.0 | MIT |
| `is-extglob` | 2.1.1 | MIT |
| `is-glob` | 4.0.3 | MIT |
| `is-number` | 7.0.0 | MIT |
| `is-what` | 4.1.16 | MIT |
| `isarray` | 1.0.0 | MIT |
| `isexe` | 2.0.0 | ISC |
| `isexe` | 3.1.1 | ISC |
| `isomorphic.js` | 0.2.5 | MIT |
| `jest-worker` | 27.5.1 | MIT |
| `jotai` | 2.11.0 | MIT |
| `jotai-scope` | 0.7.2 | MIT |
| `js-tokens` | 4.0.0 | MIT |
| `jsesc` | 3.1.0 | MIT |
| `json-parse-even-better-errors` | 4.0.0 | MIT |
| `json-schema-traverse` | 1.0.0 | MIT |
| `json5` | 2.2.3 | MIT |
| `jszip` | 3.10.1 | (MIT OR GPL-3.0-or-later) |
| `katex` | 0.16.45 | MIT |
| `khroma` | 2.1.0 | NOT-DECLARED |
| `langium` | 3.3.1 | MIT |
| `layout-base` | 1.0.2 | MIT |
| `layout-base` | 2.0.1 | MIT |
| `less` | 4.6.4 | Apache-2.0 |
| `less-loader` | 12.3.2 | MIT |
| `lib0` | 0.2.117 | MIT |
| `lie` | 3.3.0 | MIT |
| `linkifyjs` | 4.3.3 | MIT |
| `loader-runner` | 4.3.1 | MIT |
| `lodash-es` | 4.17.21 | MIT |
| `lodash-es` | 4.18.1 | MIT |
| `lodash.debounce` | 4.0.8 | MIT |
| `lodash.throttle` | 4.1.1 | MIT |
| `lowlight` | 3.3.0 | MIT |
| `lru-cache` | 5.1.1 | ISC |
| `magic-string` | 0.30.21 | MIT |
| `make-dir` | 2.1.0 | MIT |
| `marked` | 16.4.2 | MIT |
| `marked` | 17.0.5 | MIT |
| `marked-katex-extension` | 5.1.8 | MIT |
| `math-intrinsics` | 1.1.0 | MIT |
| `memorystream` | 0.3.1 | MIT |
| `merge-stream` | 2.0.0 | MIT |
| `mermaid` | 11.15.0 | MIT |
| `mime` | 1.6.0 | MIT |
| `mime-db` | 1.52.0 | MIT |
| `mime-db` | 1.54.0 | MIT |
| `mime-types` | 2.1.35 | MIT |
| `mitt` | 2.1.0 | MIT |
| `mitt` | 3.0.1 | MIT |
| `mlly` | 1.8.2 | MIT |
| `ms` | 2.1.3 | MIT |
| `muggle-string` | 0.4.1 | MIT |
| `multimath` | 2.0.0 | MIT |
| `nanoid` | 3.3.11 | MIT |
| `nanoid` | 3.3.3 | MIT |
| `nanoid` | 4.0.2 | MIT |
| `needle` | 3.3.1 | MIT |
| `neo-async` | 2.6.2 | MIT |
| `node-releases` | 2.0.27 | MIT |
| `normalize-path` | 3.0.0 | MIT |
| `npm-normalize-package-bin` | 4.0.0 | ISC |
| `npm-run-all2` | 8.0.4 | MIT |
| `object-assign` | 4.1.1 | MIT |
| `open-color` | 1.9.1 | MIT |
| `orderedmap` | 2.1.1 | MIT |
| `package-manager-detector` | 1.6.0 | MIT |
| `pagefind` | 1.5.2 | MIT |
| `pako` | 1.0.11 | (MIT AND Zlib) |
| `pako` | 2.0.3 | (MIT AND Zlib) |
| `papaparse` | 5.5.3 | MIT |
| `parse-node-version` | 1.0.1 | MIT |
| `path-browserify` | 1.0.1 | MIT |
| `path-data-parser` | 0.1.0 | MIT |
| `path-key` | 3.1.1 | MIT |
| `pathe` | 2.0.3 | MIT |
| `perfect-debounce` | 1.0.0 | MIT |
| `perfect-freehand` | 1.2.0 | MIT |
| `pica` | 7.1.1 | MIT |
| `picocolors` | 1.1.1 | ISC |
| `picomatch` | 2.3.2 | MIT |
| `picomatch` | 4.0.4 | MIT |
| `pidtree` | 0.6.0 | MIT |
| `pify` | 4.0.1 | MIT |
| `pinia` | 3.0.4 | MIT |
| `pkg-types` | 1.3.1 | MIT |
| `png-chunk-text` | 1.0.0 | MIT |
| `png-chunks-encode` | 1.0.0 | MIT |
| `png-chunks-extract` | 1.0.0 | MIT |
| `points-on-curve` | 0.2.0 | MIT |
| `points-on-curve` | 1.0.1 | MIT |
| `points-on-path` | 0.2.1 | MIT |
| `postcss` | 8.5.14 | MIT |
| `process-nextick-args` | 2.0.1 | MIT |
| `prosemirror-changeset` | 2.4.3 | MIT |
| `prosemirror-commands` | 1.7.2 | MIT |
| `prosemirror-dropcursor` | 1.8.3 | MIT |
| `prosemirror-gapcursor` | 1.4.1 | MIT |
| `prosemirror-history` | 1.5.0 | MIT |
| `prosemirror-inputrules` | 1.5.1 | MIT |
| `prosemirror-keymap` | 1.2.3 | MIT |
| `prosemirror-model` | 1.25.11 | MIT |
| `prosemirror-schema-list` | 1.5.1 | MIT |
| `prosemirror-state` | 1.4.4 | MIT |
| `prosemirror-tables` | 1.8.5 | MIT |
| `prosemirror-transform` | 1.12.1 | MIT |
| `prosemirror-view` | 1.42.4 | MIT |
| `proxy-from-env` | 2.1.0 | MIT |
| `prr` | 1.0.1 | MIT |
| `pwacompat` | 2.0.17 | Apache-2.0 |
| `react` | 19.3.0 | MIT |
| `react-dom` | 19.3.0 | MIT |
| `react-remove-scroll` | 2.7.2 | MIT |
| `react-remove-scroll-bar` | 2.3.8 | MIT |
| `react-style-singleton` | 2.2.3 | MIT |
| `read-package-json-fast` | 4.0.0 | ISC |
| `readable-stream` | 2.3.8 | MIT |
| `readdirp` | 3.6.0 | MIT |
| `require-from-string` | 2.0.2 | MIT |
| `rfdc` | 1.4.1 | MIT |
| `robust-predicates` | 3.0.3 | Unlicense |
| `rollup` | 4.59.0 | MIT |
| `rope-sequence` | 1.3.4 | MIT |
| `roughjs` | 4.6.4 | MIT |
| `roughjs` | 4.6.6 | MIT |
| `rw` | 1.3.3 | BSD-3-Clause |
| `safe-buffer` | 5.1.2 | MIT |
| `safer-buffer` | 2.1.2 | MIT |
| `sass` | 1.51.0 | MIT |
| `sax` | 1.4.1 | ISC |
| `scheduler` | 0.28.0 | MIT |
| `schema-utils` | 4.3.3 | MIT |
| `semver` | 5.7.2 | ISC |
| `semver` | 6.3.1 | ISC |
| `setimmediate` | 1.0.5 | MIT |
| `shebang-command` | 2.0.0 | MIT |
| `shebang-regex` | 3.0.0 | MIT |
| `shell-quote` | 1.10.0 | MIT |
| `sliced` | 1.0.1 | MIT |
| `sortablejs` | 1.15.6 | MIT |
| `source-map` | 0.6.1 | BSD-3-Clause |
| `source-map-js` | 1.2.1 | BSD-3-Clause |
| `source-map-support` | 0.5.21 | MIT |
| `speakingurl` | 14.0.1 | BSD-3-Clause |
| `string_decoder` | 1.1.1 | MIT |
| `stylis` | 4.3.6 | MIT |
| `superjson` | 2.2.2 | MIT |
| `supports-color` | 8.1.1 | MIT |
| `swiper` | 12.1.4 | MIT |
| `tapable` | 2.3.3 | MIT |
| `tdesign-icons-vue-next` | 0.4.4 | MIT |
| `tdesign-vue-next` | 1.19.2 | MIT |
| `terser` | 5.43.1 | BSD-2-Clause |
| `terser-webpack-plugin` | 5.4.0 | MIT |
| `tinycolor2` | 1.6.0 | MIT |
| `tinyexec` | 1.1.1 | MIT |
| `tinyglobby` | 0.2.15 | MIT |
| `to-regex-range` | 5.0.1 | MIT |
| `ts-dedent` | 2.2.0 | MIT |
| `tslib` | 2.8.1 | 0BSD |
| `tsx` | 4.23.1 | MIT |
| `tunnel-rat` | 0.1.2 | MIT |
| `typescript` | 6.0.3 | Apache-2.0 |
| `ufo` | 1.6.3 | MIT |
| `undici-types` | 6.21.0 | MIT |
| `update-browserslist-db` | 1.2.3 | MIT |
| `use-callback-ref` | 1.3.3 | MIT |
| `use-sidecar` | 1.1.3 | MIT |
| `use-sync-external-store` | 1.7.0 | MIT |
| `util-deprecate` | 1.0.2 | MIT |
| `uuid` | 14.0.1 | MIT |
| `validator` | 13.15.23 | MIT |
| `vite` | 7.3.6 | MIT |
| `vscode-jsonrpc` | 8.2.0 | MIT |
| `vscode-languageserver` | 9.0.1 | MIT |
| `vscode-languageserver-protocol` | 3.17.5 | MIT |
| `vscode-languageserver-textdocument` | 1.0.14 | MIT |
| `vscode-languageserver-types` | 3.17.5 | MIT |
| `vscode-uri` | 3.0.8 | MIT |
| `vscode-uri` | 3.1.0 | MIT |
| `vue` | 3.5.34 | MIT |
| `vue-demi` | 0.14.10 | MIT |
| `vue-i18n` | 11.4.2 | MIT |
| `vue-router` | 4.5.1 | MIT |
| `vue-tsc` | 3.2.8 | MIT |
| `vue-virtual-scroller` | 2.0.1 | MIT |
| `w3c-keyname` | 2.2.8 | MIT |
| `watchpack` | 2.5.1 | MIT |
| `webpack` | 5.106.2 | MIT |
| `webpack-sources` | 3.4.1 | MIT |
| `webworkify` | 1.5.0 | MIT |
| `which` | 2.0.2 | ISC |
| `which` | 5.0.0 | ISC |
| `xlsx` | 0.20.2 | Apache-2.0 |
| `y-indexeddb` | 9.0.12 | MIT |
| `y-protocols` | 1.0.7 | MIT |
| `yallist` | 3.1.1 | ISC |
| `yjs` | 13.6.32 | MIT |
| `zustand` | 4.5.7 | MIT |

</details>

---

## npm packages (collab)

The collaboration service (`collab/`) is a separate Node application with its own
`package.json` and container image; these are its runtime and build dependencies.

| License | Packages |
|---|---|
| MIT | 31 |
| Apache-2.0 | 3 |
| 0BSD | 1 |

<details>
<summary>Full list (35 packages)</summary>

| Package | Version | License |
|---|---|---|
| `@esbuild/win32-x64` | 0.28.2 | MIT |
| `@hocuspocus/common` | 4.7.0 | MIT |
| `@hocuspocus/provider` | 4.7.0 | MIT |
| `@hocuspocus/server` | 4.7.0 | MIT |
| `@ioredis/commands` | 1.10.0 | MIT |
| `@lifeomic/attempt` | 3.1.0 | MIT |
| `@types/node` | 24.13.5 | MIT |
| `@types/ws` | 8.18.1 | MIT |
| `async-mutex` | 0.5.0 | MIT |
| `cluster-key-slot` | 1.1.1 | Apache-2.0 |
| `crossws` | 0.4.12 | MIT |
| `debug` | 4.4.3 | MIT |
| `denque` | 2.1.0 | Apache-2.0 |
| `esbuild` | 0.28.2 | MIT |
| `ioredis` | 5.11.1 | MIT |
| `isomorphic.js` | 0.2.5 | MIT |
| `kleur` | 4.1.5 | MIT |
| `lib0` | 0.2.117 | MIT |
| `ms` | 2.1.3 | MIT |
| `orderedmap` | 2.1.1 | MIT |
| `prosemirror-model` | 1.25.11 | MIT |
| `prosemirror-state` | 1.4.4 | MIT |
| `prosemirror-transform` | 1.12.1 | MIT |
| `prosemirror-view` | 1.42.3 | MIT |
| `redis-errors` | 1.2.0 | MIT |
| `redis-parser` | 3.0.0 | MIT |
| `standard-as-callback` | 2.1.0 | MIT |
| `tslib` | 2.8.1 | 0BSD |
| `tsx` | 4.23.13 | MIT |
| `typescript` | 5.9.3 | Apache-2.0 |
| `undici-types` | 7.18.2 | MIT |
| `ws` | 8.21.3 | MIT |
| `y-prosemirror` | 1.3.7 | MIT |
| `y-protocols` | 1.0.7 | MIT |
| `yjs` | 13.6.32 | MIT |

</details>

---

## Python distributions (docreader)

| License | Distributions |
|---|---|
| MIT License | 14 |
| BSD License | 12 |
| MIT | 10 |
| Apache Software License | 7 |
| BSD-3-Clause | 6 |
| Apache-2.0 | 5 |
| 3-Clause BSD License | 1 |
| Apache Software License; BSD License | 1 |
| Apache-2.0 AND CNRI-Python | 1 |
| Apache-2.0 OR BSD-3-Clause | 1 |
| BSD License; Apache Software License | 1 |
| BSD-3-Clause, Apache-2.0, dependency licenses | 1 |
| MIT AND Python-2.0 | 1 |
| MIT-CMU | 1 |
| MPL-1.1 OR GPL-2.0-only OR LGPL-2.1-or-later | 1 |
| Mozilla Public License 2.0 (MPL 2.0) | 1 |
| PSF-2.0 | 1 |
| Python Software Foundation License | 1 |

<details>
<summary>Full list (66 distributions)</summary>

| Distribution | Version | License |
|---|---|---|
| `annotated-types` | 0.7.0 | MIT License |
| `babel` | 2.17.0 | BSD License |
| `beautifulsoup4` | 4.14.2 | MIT License |
| `certifi` | 2025.10.5 | Mozilla Public License 2.0 (MPL 2.0) |
| `cffi` | 2.0.0 | MIT |
| `charset-normalizer` | 3.4.4 | MIT |
| `click` | 8.3.0 | BSD-3-Clause |
| `cobble` | 0.1.4 | BSD License |
| `coloredlogs` | 15.0.1 | MIT License |
| `courlan` | 1.3.2 | Apache Software License |
| `cryptography` | 49.0.0 | Apache-2.0 OR BSD-3-Clause |
| `dateparser` | 1.2.2 | BSD License |
| `defusedxml` | 0.7.1 | Python Software Foundation License |
| `et_xmlfile` | 2.0.0 | MIT License |
| `flatbuffers` | 25.9.23 | Apache Software License |
| `greenlet` | 3.2.4 | MIT AND Python-2.0 |
| `grpcio` | 1.80.0 | Apache-2.0 |
| `grpcio-health-checking` | 1.80.0 | Apache-2.0 |
| `grpcio-tools` | 1.80.0 | Apache-2.0 |
| `htmldate` | 1.9.4 | Apache Software License |
| `humanfriendly` | 10.0 | MIT License |
| `idna` | 3.18 | BSD-3-Clause |
| `jusText` | 3.0.2 | BSD License |
| `lxml` | 6.1.0 | BSD-3-Clause |
| `lxml_html_clean` | 0.4.4 | BSD-3-Clause |
| `magika` | 0.6.3 | Apache Software License |
| `mammoth` | 1.11.0 | BSD License |
| `markdownify` | 1.2.0 | MIT License |
| `markitdown` | 0.1.3 | MIT |
| `mpmath` | 1.3.0 | BSD License |
| `numpy` | 2.2.6 | BSD License |
| `onnxruntime` | 1.20.1 | MIT License |
| `opendataloader-pdf` | 2.4.7 | Apache-2.0 |
| `openpyxl` | 3.1.5 | MIT License |
| `packaging` | 25.0 | Apache Software License; BSD License |
| `pandas` | 2.3.3 | BSD License |
| `pdfminer.six` | 20250506 | MIT |
| `pillow` | 12.3.0 | MIT-CMU |
| `playwright` | 1.55.0 | Apache-2.0 |
| `protobuf` | 6.33.6 | 3-Clause BSD License |
| `pycparser` | 2.23 | BSD License |
| `pydantic` | 2.13.4 | MIT |
| `pydantic_core` | 2.46.4 | MIT |
| `pyee` | 13.0.0 | MIT License |
| `pypdf` | 6.14.2 | BSD-3-Clause |
| `pypdfium2` | 5.8.0 | BSD-3-Clause, Apache-2.0, dependency licenses |
| `python-dateutil` | 2.9.0.post0 | BSD License; Apache Software License |
| `python-docx` | 1.2.0 | MIT License |
| `python-dotenv` | 1.2.2 | BSD-3-Clause |
| `python-pptx` | 1.0.2 | MIT License |
| `pytz` | 2025.2 | MIT License |
| `regex` | 2025.11.3 | Apache-2.0 AND CNRI-Python |
| `requests` | 2.34.2 | Apache Software License |
| `setuptools` | 80.9.0 | MIT |
| `six` | 1.17.0 | MIT License |
| `soupsieve` | 2.8.4 | MIT |
| `sympy` | 1.14.0 | BSD License |
| `tld` | 0.13.1 | MPL-1.1 OR GPL-2.0-only OR LGPL-2.1-or-later |
| `trafilatura` | 2.0.0 | Apache Software License |
| `typing-inspection` | 0.4.2 | MIT |
| `typing_extensions` | 4.15.0 | PSF-2.0 |
| `tzdata` | 2025.2 | Apache Software License |
| `tzlocal` | 5.3.1 | MIT License |
| `urllib3` | 2.7.0 | MIT |
| `xlrd` | 2.0.2 | BSD License |
| `xlsxwriter` | 3.2.9 | BSD License |

</details>

---

## Python distributions (mcp-server)

| License | Distributions |
|---|---|
| MIT | 21 |
| BSD-3-Clause | 11 |
| MIT License | 9 |
| Apache-2.0 | 4 |
| Mozilla Public License 2.0 (MPL 2.0) | 1 |
| MIT-0 | 1 |
| Apache-2.0 OR BSD-3-Clause | 1 |
| MIT AND PSF-2.0 | 1 |
| BSD License | 1 |
| BSD License; Apache Software License | 1 |
| MIT OR Apache-2.0 | 1 |
| Apache Software License | 1 |
| PSF-2.0 | 1 |
| Apache Software License; MIT License | 1 |

<details>
<summary>Full list (55 distributions)</summary>

| Distribution | Version | License |
|---|---|---|
| `annotated-doc` | 0.0.5 | MIT |
| `annotated-types` | 0.8.0 | MIT |
| `anyio` | 4.15.1 | MIT |
| `asyncpg` | 0.31.0 | Apache-2.0 |
| `attrs` | 26.1.0 | MIT |
| `botocore` | 1.43.95 | Apache-2.0 |
| `certifi` | 2026.7.22 | Mozilla Public License 2.0 (MPL 2.0) |
| `cffi` | 2.1.1 | MIT-0 |
| `click` | 8.5.0 | BSD-3-Clause |
| `cryptography` | 50.0.1 | Apache-2.0 OR BSD-3-Clause |
| `fastapi` | 0.141.1 | MIT |
| `greenlet` | 3.5.6 | MIT AND PSF-2.0 |
| `h11` | 0.16.0 | MIT License |
| `h2` | 4.4.1 | MIT |
| `hpack` | 4.2.0 | MIT |
| `httpcore` | 1.0.9 | BSD-3-Clause |
| `httpcore2` | 2.13.0 | BSD-3-Clause |
| `httptools` | 0.8.0 | MIT |
| `httpx` | 0.28.1 | BSD License |
| `httpx2` | 2.13.0 | BSD-3-Clause |
| `hyperframe` | 6.1.0 | MIT License |
| `idna` | 3.19 | BSD-3-Clause |
| `jmespath` | 1.1.0 | MIT License |
| `jsonschema` | 4.26.0 | MIT |
| `jsonschema-specifications` | 2025.9.1 | MIT |
| `mcp` | 2.2.0 | MIT License |
| `mcp-types` | 2.2.0 | MIT License |
| `opentelemetry-api` | 1.44.0 | Apache-2.0 |
| `pip` | 25.0.1 | MIT License |
| `pycparser` | 3.0 | BSD-3-Clause |
| `pydantic` | 2.13.5 | MIT |
| `pydantic-settings` | 2.15.0 | MIT |
| `pydantic_core` | 2.46.5 | MIT |
| `PyJWT` | 2.14.0 | MIT |
| `python-dateutil` | 2.9.0.post0 | BSD License; Apache Software License |
| `python-dotenv` | 1.2.3 | BSD-3-Clause |
| `python-multipart` | 0.0.32 | Apache-2.0 |
| `PyYAML` | 6.0.3 | MIT License |
| `redis` | 8.1.0 | MIT |
| `referencing` | 0.37.0 | MIT |
| `rpds-py` | 2026.6.3 | MIT |
| `six` | 1.17.0 | MIT License |
| `SQLAlchemy` | 2.0.54 | MIT |
| `sse-starlette` | 3.4.11 | BSD-3-Clause |
| `starlette` | 1.6.0 | BSD-3-Clause |
| `structlog` | 26.1.0 | MIT OR Apache-2.0 |
| `tenacity` | 9.1.4 | Apache Software License |
| `truststore` | 0.10.4 | MIT |
| `typing-inspection` | 0.4.4 | MIT |
| `typing_extensions` | 4.16.0 | PSF-2.0 |
| `urllib3` | 2.8.0 | MIT |
| `uvicorn` | 0.53.0 | BSD-3-Clause |
| `uvloop` | 0.22.1 | Apache Software License; MIT License |
| `watchfiles` | 1.2.0 | MIT License |
| `websockets` | 17.1 | BSD-3-Clause |

</details>
