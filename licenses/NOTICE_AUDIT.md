# License & Attribution Audit

Audit date: **2026-09-17**
Subject: the Yuheng working tree at the point of its first independent import,
forked from `Tencent/WeKnora` (common ancestor `a753a153`, 2026-08-25).

This file records what was examined, what was changed, what was deliberately
left alone, and what still blocks a release. It is the companion to
[`../NOTICE`](../NOTICE) and [`../THIRD_PARTY_NOTICES.md`](../THIRD_PARTY_NOTICES.md).

---

## 1. Blocking items — all resolved on 2026-09-18

Each of these was a dependency whose license is incompatible with distributing
Yuheng under MIT. They were **inherited from upstream**, not introduced by the
fork; upstream's own `LICENSE` scan did not surface them. None remains; the
sections are kept so the reasoning and the replacements stay auditable.

### 1.1 `github.com/liuzl/cedar-go` — GPL-2.0 — **linked into every server build** — **RESOLVED 2026-09-18**

`longbridgeapp/opencc` (and with it `liuzl/da` and `liuzl/cedar-go`) left
`go.mod`. OpenCC's own `t2s` data — `TSPhrases.txt` and `TSCharacters.txt`,
Apache-2.0, copied unmodified into `internal/types/opencc/` with their LICENSE
and a provenance README — is now embedded and applied by `internal/types/t2s.go`
with the same maximum-matching semantics (phrase table first, character table
as fallback). Conversion results are therefore unchanged (t2s is a script
conversion, not a Taiwan-vocabulary one, so 軟體 still becomes 软体 as before);
only the GPL-licensed trie implementation is gone. A public-domain character table (`gojianfan`) was
evaluated first and rejected: it left common characters such as 繫 unconverted
and drifted already-simplified text (案→桉). The analysis below is kept for the
record.

```
github.com/magicyuan876/yuheng/internal/types
  └─ github.com/longbridgeapp/opencc        (Apache-2.0)
       └─ github.com/liuzl/da               (Apache-2.0)
            └─ github.com/liuzl/cedar-go    (GPL-2.0, LICENSE.md)
```

`internal/types` is imported by essentially every package, so this is not a
build-tag-gated corner: the GPL-2.0 object code ends up in the shipped binary.
Distributing that binary under MIT is not
something the GPL-2.0 permits.

The only consumer is `internal/types/faq.go`, which builds one
traditional→simplified Chinese converter:

```go
t2sConverter, err = opencc.New("t2s")   // internal/types/faq.go:610
```

Options, cheapest first:

1. Replace `longbridgeapp/opencc` with a T2S converter that does not pull in a
   double-array trie — e.g. a table-driven mapping, or `go-ego/gse`'s own
   conversion path (Apache-2.0), or drop the normalisation and match on both
   forms at query time.
2. Vendor a permissively licensed double-array trie in place of `liuzl/da`.
3. Drop traditional-Chinese FAQ normalisation entirely.

A variant of option 1 was taken (embed OpenCC's own Apache-2.0 tables); see the resolution note above.

### 1.2 `EbookLib` 0.20 — AGPL-3.0-or-later — docreader (Python) — **RESOLVED 2026-09-18**

`docreader/parser/epub_parser.py` now reads the EPUB container directly with the
standard library: `META-INF/container.xml` → OPF `<manifest>`/`<spine>` for
reading order, Dublin Core metadata from the OPF, XHTML → Markdown through the
existing BeautifulSoup + markdownify path. Archives without a usable OPF fall
back to scanning for XHTML members. `ebooklib` was removed from
`docreader/pyproject.toml` and the tests build their fixture EPUB with
`zipfile`, so nothing in the tree imports it any more.

### 1.3 `chardet` 5.2.0 — LGPL-2.1-or-later — docreader (Python) — **RESOLVED 2026-09-18**

`chardet` was never imported by docreader. It arrived as a transitive dependency
of `textract==1.5.0`, whose only call site (`doc_parser.py::_parse_with_textract`)
had already been disabled for SSRF reasons. `textract` was removed from
`docreader/pyproject.toml`, which also dropped `argcomplete`, `docx2txt`,
`SpeechRecognition` and the `standard-*` shims from the lock. No replacement was
needed; `requests` already ships `charset-normalizer` (MIT) for encoding
detection.

---

## 2. Items needing a decision, but not blocking

| Item | Issue | Suggested handling |
|---|---|---|
| `frontend/src/assets/fonts/TencentSans.ttf` | Was a Tencent typeface shipped in the repo, not covered by the MIT license. | **Resolved 2026-09-17:** the `@font-face` was never referenced by any stylesheet, so the file and `fonts.css` were deleted. No replacement needed. |
| `tld` 0.13.1 (Python) | MPL-1.1 OR GPL-2.0-only OR LGPL-2.1-or-later | Elect **MPL-1.1**; record the election (done in THIRD_PARTY_NOTICES.md). |
| `docx2txt` 0.9 (Python) | No license declared in package metadata | **Resolved 2026-09-18:** no longer a dependency; it left the lock together with `textract` (§1.3). |
| `caniuse-lite` (npm) | CC-BY-4.0 — attribution required | Attribution given in THIRD_PARTY_NOTICES.md. Build-time only; not in the shipped bundle. |
| `dompurify` (npm) | MPL-2.0 OR Apache-2.0 | Elected **Apache-2.0**; recorded. |
| 4 Go modules unresolved offline | `danieljoos/wincred`, `erikgeiser/coninput`, `inconshreveable/mousetrap`, `mattn/go-localereader` were absent from the module cache used for the first scan | **Resolved 2026-09-30:** they are `cli/` dependencies, and only the root module's had been downloaded. `go mod download` fetches every required module whatever the GOOS, so running it in `.`, `cli/` and `client/` (as CI does) makes the scan complete. Three classify normally; `go-localereader` ships no LICENSE file at all — its README states MIT, which the notices record through the generator's annotation map. |
| `lightningcss` (npm, 12 entries with its platform binaries) | MPL-2.0; arrived with Tailwind v4 as its CSS compiler | Build-time only and used unmodified; nothing MPL-covered is in the shipped bundle. Recorded in THIRD_PARTY_NOTICES.md. |
| `@esbuild/*` 0.25.6 binaries, `fsevents` (npm) | Platform-specific packages whose license `frontend/package-lock.json` does not record; the generator reads platform-specific licenses from the lockfile only (so the list is the same on every OS) and shows them as UNRESOLVED | All are MIT per their own `package.json`, and build/dev-time only; recorded in THIRD_PARTY_NOTICES.md. A future lockfile refresh that records the field clears them. |
| `pypdfium2` (Python, docreader) | Declares "BSD-3-Clause, Apache-2.0, dependency licenses": the wheel bundles a PDFium build with FreeType, ICU, libjpeg-turbo, libpng, libtiff, Little CMS, OpenJPEG, zlib, abseil and others | All permissive; texts ship in the wheel's `dist-info/licenses/`. FreeType is dual FTL / GPL-2.0 — FTL elected, recorded in THIRD_PARTY_NOTICES.md. |
| `third_party/anydoc-go/` static libraries | The vendored Go bindings link `libanydoc_go`, a Rust library built from the vendored crate; its `Cargo.lock` pins 168 packages, and no scanner covered Cargo | **Resolved 2026-09-30:** `scan_cargo.py` (§5) scans the lockfile and THIRD_PARTY_NOTICES.md lists every crate. Nothing denied. All but four crates offer MIT, two of them with a permissive term that applies alongside it (`encoding_rs` BSD-3-Clause, `unicode-ident` Unicode-3.0); of the four, `zlib-rs` (Zlib), `zopfli` (Apache-2.0) and `ryu` (Apache-2.0 OR BSL-1.0; BSL-1.0 elected) are permissive, and `cbindgen` is MPL-2.0 but build-time only, generating the C header. `r-efi` offers LGPL-2.1 next to MIT and Apache-2.0 and is UEFI-only. Elections recorded in THIRD_PARTY_NOTICES.md. |
| mcp-server image | `mcp-server/Dockerfile` installs `requirements.txt` (version ranges) with pip, not `uv.lock`, so the image can resolve other versions than the ones THIRD_PARTY_NOTICES.md lists | Build the image from the lock (`uv export --frozen --no-dev`, as docreader does) so the notices describe what ships. |
| `packages/dsh-yuheng`, `packages/docs-schema`, `website-docs` npm trees | Not scanned | Scan before publishing anything from those directories. |

---

## 3. Three-way classification of every `WeKnora` / `Tencent` occurrence

The rename was driven by this classification, applied per *shape* of occurrence
rather than per file.

### 3.1 `legal-do-not-touch` — left byte-for-byte unchanged

| What | Where | Why |
|---|---|---|
| Tencent's MIT text + the upstream third-party notice bundle | `licenses/upstream-weknora/LICENSE` (161 KB) | The MIT License's one condition. Removing or editing it is the one thing that would actually make the fork non-compliant. It sat at the repository root until 2026-10-08 and was then moved, byte for byte, so that the root `LICENSE` can be the plain MIT text that license detectors (GitHub's among them) recognise; the root file keeps Tencent's copyright line beside ours. Moving the file within the distribution changes nothing about the obligation — it ships wherever `licenses/` ships. |
| `Copyright (c) 2024 WeKnora Team` | `mcp-server/LICENSE` | A copyright line. (The mechanical pass did rewrite this one; it was **restored**, with a `Modifications Copyright` line added beneath.) |
| `Copyright (c) 2026 Sideguide Technologies Inc.` | `third_party/anydoc-go/LICENSE` | Vendored dependency's own license. |
| Upstream issue/PR references `Tencent/WeKnora#2136`, `#1262`, `#2768` | 10 Go files, 1 Python test, 1 doc | Attribution facts. They say "this code exists because of that upstream bug report", and that is true and useful. Protected with a sentinel through every mechanical pass. |
| Release history in `CHANGELOG.md`, `cli/CHANGELOG.md`, `mcp-server/CHANGELOG.md` | whole files | A record of what happened, including links to upstream PRs that only exist upstream. Rewriting the URLs would make them wrong. |
| Git history | all commits | Not rewritten, not force-pushed. Every upstream commit keeps its original author. |
| `Copyright 2025 Tencent` headers | `helm/templates/*.yaml`, `helm/templates/NOTES.txt`, `helm/templates/_helpers.tpl`, `helm/values.yaml` | Existing source copyright headers. **Kept**, and extended per §4 below. |

### 3.2 `code-dependency` — renamed, but the rename changes runtime behaviour

These are not cosmetic. Each one breaks something on an existing deployment
unless the operator acts; they are listed in the release checklist (§6).

| Identifier | Old | New | Breaks |
|---|---|---|---|
| Go module path | `github.com/Tencent/WeKnora[/cli,/client,…]` | `github.com/magicyuan876/yuheng[…]` | Import paths in 1 366 Go files; all four `go.mod` files |
| Environment variables | `WEKNORA_*` (90 names, 922 occurrences) | `YUHENG_*` | **Every existing `.env`.** A stale variable is silently ignored. No deployment predates the rename (every one is recreated from `.env.example`), so the startup warning that once caught stale names was removed |
| Vector collection / table | `weknora_embeddings`, `weknora_embeddings_{768,1024}` | `yuheng_embeddings*` | Existing pgvector / Qdrant / Milvus / Weaviate / Doris data becomes invisible; needs a rename or re-index |
| Postgres database name | `weknora` | `yuheng` | `DB_NAME` in `.env`; existing volumes |
| Redis key namespace | `weknora:*` | `yuheng:*` | Cache and sandbox-session keys; cold start after upgrade |
| Browser storage keys | `weknora_token`, `weknora_refresh_token`, `weknora_selected_tenant_id`, `WeKnora_settings` | `yuheng_*`, `Yuheng_settings` | All users are logged out once |
| CLI config paths | `~/.config/weknora/config.yaml`, `.weknora/project.yaml` | `~/.config/yuheng/…`, `.yuheng/project.yaml` | Existing CLI profiles and per-directory links |
| MCP tool names | `weknora_search`, `weknora_ask`, `weknora_read_document`, `weknora_list_knowledge_bases` | `yuheng_*` | Any agent configuration that names the tools |
| Container images | `wechatopenai/weknora-{app,ui,docreader,sandbox,docs}` | `magicyuan876/yuheng-*` | Not yet published — see checklist |
| PyPI package | `tencent-weknora-mcp` | `yuheng-mcp` | Not yet published — name availability unverified |
| npm package | `@wxg-prc-cpg/dsh-weknora` | `@magicyuan876/dsh-yuheng` | Not yet published — scope ownership unverified |
| Docker network / containers | `WeKnora-network`, `WeKnora-*` | `Yuheng-network`, `Yuheng-*` | `docker compose down` before upgrading, or orphan containers remain |
| systemd units | `weknora.service`, `weknora-firstboot.service` | `yuheng.service`, `yuheng-firstboot.service` | Cloud-image firstboot; existing installs need the unit re-enabled |
| Homebrew formula | `Formula/weknora-lite.rb` | `Formula/yuheng-lite.rb` | Tap users must re-install |

### 3.3 `brand-ok-to-replace` — pure naming, no behavioural effect

Display strings, log messages, comments, documentation prose, Vue/Go type names
(`WeKnoraClient` → `YuhengClient`, `WeKnoraDesktopWindow` → `YuhengDesktopWindow`),
i18n values, README/website copy, image alt text, example hostnames
(`weknora.example.com` → `yuheng.example.com`), test fixtures and AES key
placeholders. 349 Go files and 305 non-Go files.

---

## 4. Modification notices added

Every file that carried a `Copyright 2025 Tencent` header now carries the
project's standard three-line notice instead of the original two:

```
Portions Copyright (c) 2025 Tencent.
Modifications Copyright (c) 2026 magicyuan876.
SPDX-License-Identifier: MIT
```

Applied to: `helm/values.yaml`, `helm/templates/NOTES.txt`,
`helm/templates/_helpers.tpl`, and `helm/templates/{app,docreader,frontend,
ingress,neo4j,postgres,pvc,redis,secrets,serviceaccount}.yaml` — 13 files.

`mcp-server/LICENSE` keeps `Copyright (c) 2024 WeKnora Team` and gains
`Modifications Copyright (c) 2026 magicyuan876` beneath it.

**Apache-2.0 §4(b)**: no Apache-2.0 licensed file is modified in place anywhere
in this tree — every Apache-2.0 dependency is consumed unmodified from its
registry — so no per-file change notices are required.

**MPL-2.0 §3.2**: likewise, no MPL-2.0 covered file is modified. The MPL
components (`cbindgen`, `certifi`, `dompurify`, `go-sql-driver/mysql`,
`lightningcss`, `tld`) are used as published;
their sources remain available at the upstream URLs recorded in
THIRD_PARTY_NOTICES.md.

---

## 5. Method, and what this audit does not cover

**How the inventories are built.** `tools/license_check.sh` runs one scanner per
ecosystem, from `tools/licensescan/`, and each writes JSON:

* **Go** (`collect_gomods.py`, `scan_go.py`) — every `go.mod` that `git ls-files`
  finds is parsed with `go mod edit -json`; `replace` directives are applied, so
  a module replaced by another version is read at the replacement and one
  replaced by a local directory (`third_party/anydoc-go`, the sibling `client`
  module) is read from the working tree. Each module's `LICENSE`/`COPYING`/
  `NOTICE` file is located in the module cache and matched against SPDX
  signatures; submodule paths (`.../v18`, `.../lib/linux-arm64`) fall back to the
  repository root's license. A module of this repository takes the license
  `NOTICE` declares.
* **Cargo** (`scan_cargo.py`) — the package list is `third_party/anydoc-go/Cargo.lock`,
  the lockfile `scripts/build-anydoc-lib.sh` builds with `--locked`. Each crate
  version's `license` expression is read from the crates.io API, and its
  dependency kinds and targets from the crates.io index, whose checksum must
  match the lockfile's; no Rust toolchain is needed, and every answer is cached
  per name@version, since a published version never changes. Like the npm scan,
  the list is the whole lockfile, build tooling and other platforms' crates
  included, so a copyleft crate anywhere in it fails the gate; each row also
  says whether the crate is linked into the library for linux/amd64 and
  linux/arm64 (the targets the image builds, `cfg(...)` predicates evaluated for
  each), only compiled to run at build time (`cbindgen` and build scripts), or
  not built for Linux at all (the `windows-*` family, wasm, UEFI). Proc-macro
  crates count as linked, because registry metadata does not mark them. The
  root crate `anydoc-go` is this repository's vendored code and is not listed;
  `anydoc`, which `[patch.crates-io]` replaces with a copy of the published
  crate, takes that crate's license. A git or alternative-registry source would
  be reported as unresolved for a human to classify.
* **npm** (`scan_npm.py`) — the package list is `package-lock.json`, for
  `frontend/` and `collab/` separately, not the installed tree: npm installs only
  the platform binaries that match the machine, so a list built from
  `node_modules` differs between macOS and Linux. Licenses come from each
  installed `package.json` (`license` / `licenses`); for platform-specific
  packages, from the lockfile only. A `node_modules` that does not match the
  lockfile stops the scan.
* **Python** (`scan_python.py`) — the runtime dependencies in `uv.lock` (no dev
  group, not the project itself) are exported and installed with `uv pip install
  --target --python-platform` for linux/amd64 and linux/arm64 and for the Python
  version of the image's `FROM` line, and `importlib.metadata` reads each wheel's
  `License-Expression`, then its `License ::` classifiers, then the free-text
  `License` field. Resolving for the published images rather than the local venv
  matters: docreader locks a different onnxruntime on macOS and on arm64 than on
  x86-64 Linux.

**The notices are generated from that JSON, and CI checks them.**
`tools/licensescan/render_notices.py` renders each ecosystem's license summary
and full list into `THIRD_PARTY_NOTICES.md`, between `<!-- BEGIN GENERATED:
<section> -->` / `<!-- END GENERATED: <section> -->` markers; the introduction,
the bundled assets and the licence elections stay hand-written. Facts no scanner
can see (a module with no license file) sit in a small annotation map in the
renderer, which fails if an entry stops matching the scan. The hand-kept
snapshot this replaced had drifted to dozens of modules no longer in any
`go.mod`, stale versions, and missing dependencies.

* `tools/license_check.sh notices [section…]` regenerates every section whose
  scan can run on the machine and leaves the others untouched (no module cache,
  no `node_modules`, no `uv`, crates.io unreachable with a cold cache → skipped
  with a message; in CI an unreachable crates.io fails instead).
* `tools/license_check.sh deps [section…]` scans, fails on a new copyleft or
  source-available license, and fails when a scanned section no longer matches
  the file, printing the diff and naming the `notices` command to run. A dual license passes only when it has no
  `AND`: `(MIT OR Apache-2.0) AND LGPL-2.1` keeps its LGPL term whatever is
  elected, and crates.io serves compound expressions like that routinely. In
  `.github/workflows/license.yml` the Go + npm job checks `go`, `cargo-anydoc`
  (with the crates.io answers kept in `actions/cache`), `npm-frontend` and
  `npm-collab`; the docreader and mcp-server jobs each check their own
  Python section.

The component counts live in the generated sections only; nothing in this audit
or in the notices' summary is a hand-maintained number.

**Two false positives were found and corrected in the scanner**, which is worth
recording because both would have produced a wrong blocking list:

* MPL-2.0's own text names GPL/LGPL/AGPL as "Secondary Licenses", so an AGPL
  signature tested before MPL matches *every* MPL-2.0 file. This mis-filed
  `go-sql-driver/mysql` as AGPL-3.0 on the first pass.
* `shoenig/go-m1cpu` writes its header as "Mozilla Public License, version 2.0"
  (comma, lowercase v), which a literal "Version 2.0" pattern misses.

**Two entries in the upstream `LICENSE` were re-checked and do not apply here:**

* `github.com/leodido/go-urn-1.4.0` is listed upstream under `nolicense`
  ("license not found"). It ships an **MIT** LICENSE — `Copyright (c) 2018
  Leonardo Di Donato` — visible in the module cache. Upstream's scanner missed it.
* `docx-0.2.4` is listed upstream under `unknown`. It is **not a dependency of
  this tree**: docreader uses `python-docx` 1.2.0 (MIT). The
  entry is stale from an older upstream dependency set.

**Not covered by this audit:**

* Transitive licenses of the base container images (`debian:12.12-slim`,
  `paradedb/paradedb`, `redis`, `minio`, …). Those are separately
  licensed artifacts, not part of this source tree.
* The npm trees under `website-docs/`, `packages/dsh-yuheng/` and
  `packages/docs-schema/`.
* Patent grants, trademark policy, and export-control classification.
* Whether any Chrome-extension, mini-program or IM-platform developer agreement
  imposes obligations on a renamed fork.

This audit was produced by an automated scan plus manual review. It is a
good-faith engineering record, not legal advice.

---

## 6. Pre-release checklist

**Legal — must be green**

- [x] Resolve blocking item 1.1 (`cedar-go`, GPL-2.0 in every binary) — 2026-09-18
- [x] Resolve blocking item 1.2 (`EbookLib`, AGPL-3.0) — 2026-09-18
- [x] Resolve blocking item 1.3 (`chardet`, LGPL-2.1) — 2026-09-18
- [x] `TencentSans.ttf` removed (unused `@font-face`; deleted together with `fonts.css`)
- [x] Complete Go scan — `go mod download` in every module is GOOS-independent (§2), and CI runs it
- [x] Scan the Rust crates linked into `third_party/anydoc-go` (§2) — 2026-09-30, `scan_cargo.py`
- [ ] Confirm `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`, `licenses/` are in
      the source tarball, in the container images, and in any binary archive

**Naming — must be owned before publishing**

- [ ] `github.com/magicyuan876/yuheng` created, and the module path resolves
- [ ] Docker Hub `magicyuan876/yuheng-{app,ui,docreader,sandbox,docs}` created
- [ ] PyPI name `yuheng-mcp` available and claimed
- [ ] npm scope `@magicyuan876` owned (for `@magicyuan876/dsh-yuheng`)
- [ ] Nothing in the tree claims affiliation with, or endorsement by, Tencent —
      `grep -rin "官方\|official" README*.md website-docs/` for a last look

**Operational — for anyone upgrading an existing deployment**

- [ ] `.env`: rename every `WEKNORA_*` key to `YUHENG_*` (a stale name is
      ignored silently; the server no longer warns about it)
- [ ] `.env`: `DB_NAME` `weknora` → `yuheng`, or keep the old value explicitly
- [ ] Vector store: rename or re-index `weknora_embeddings*` collections
- [ ] `SYSTEM_AES_KEY` / `JWT_SECRET`: confirm they are not the published example
      values — if they are, every stored credential is decryptable by anyone who
      read the repo
- [ ] `docker compose down` before the first `up` with the new container names
- [ ] Warn users that they will be logged out once (browser storage keys changed)
- [ ] CLI users: `~/.config/weknora/` → `~/.config/yuheng/`, `.weknora/` →
      `.yuheng/`
- [ ] Agent configs: MCP tool names `weknora_*` → `yuheng_*`
