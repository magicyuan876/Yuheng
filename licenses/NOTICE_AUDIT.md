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
| `docx2txt` 0.9 (Python) | No license declared in package metadata | Upstream repo states MIT; confirm and pin, or replace with `python-docx` (already a dependency). |
| `caniuse-lite` (npm) | CC-BY-4.0 — attribution required | Attribution given in THIRD_PARTY_NOTICES.md. Build-time only; not in the shipped bundle. |
| `dompurify` (npm) | MPL-2.0 OR Apache-2.0 | Elected **Apache-2.0**; recorded. |
| 4 Go modules unresolved offline | `danieljoos/wincred`, `erikgeiser/coninput`, `inconshreveable/mousetrap`, `mattn/go-localereader` are Windows/terminal-only and absent from the Linux module cache used for the scan | Re-run `tools/license_check.sh` on a machine with the full module cache (`GOOS=windows go mod download`) before a release that ships Windows binaries. |
| `packages/dsh-yuheng`, `website-docs`, `miniprogram` npm trees | Not installed, so not scanned | Scan before publishing anything from those directories. |

---

## 3. Three-way classification of every `WeKnora` / `Tencent` occurrence

The rename was driven by this classification, applied per *shape* of occurrence
rather than per file.

### 3.1 `legal-do-not-touch` — left byte-for-byte unchanged

| What | Where | Why |
|---|---|---|
| Tencent's MIT text + the upstream third-party notice bundle | `LICENSE` (161 KB) | The MIT License's one condition. Removing or editing it is the one thing that would actually make the fork non-compliant. |
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
| Environment variables | `WEKNORA_*` (90 names, 922 occurrences) | `YUHENG_*` | **Every existing `.env`.** A stale variable is silently ignored — a startup warning was added (`internal/config/config.go`, `warnLegacyEnvPrefix`) so it is not silent in practice |
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
components (`certifi`, `dompurify`, `go-sql-driver/mysql`, `shoenig/go-m1cpu`,
`hashicorp/errwrap`, `hashicorp/go-multierror`, `tld`) are used as published;
their sources remain available at the upstream URLs recorded in
THIRD_PARTY_NOTICES.md.

---

## 5. Method, and what this audit does not cover

**How the inventories were built.** `tools/license_check.sh` is the reproducible
form of what was run here:

* **Go** — the module list is parsed out of all four `go.mod` files (383 unique
  modules), then each module's `LICENSE`/`COPYING`/`NOTICE` file is located in
  the module cache and matched against SPDX signatures. Submodule paths
  (`.../v18`, `.../lib/linux-arm64`) fall back to the repository root's license.
* **npm** — every `package.json` under `frontend/node_modules` (382 packages),
  reading `license` / `licenses`.
* **Python** — `importlib.metadata` inside the built `docreader` (72 dists) and
  `mcp-server` (55 dists) images, preferring `License-Expression`, then
  `License ::` classifiers, then the free-text `License` field.

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
  this tree**: docreader uses `python-docx` 1.2.0 (MIT) and `docx2txt` 0.9. The
  entry is stale from an older upstream dependency set.

**Not covered by this audit:**

* Transitive licenses of the base container images (`debian:12.12-slim`,
  `paradedb/paradedb`, `redis`, `minio`, `qdrant`, …). Those are separately
  licensed artifacts, not part of this source tree.
* The npm trees under `website-docs/`, `packages/dsh-yuheng/` and `miniprogram/`
  — not installed on the audit machine.
* Patent grants, trademark policy, and export-control classification.
* Whether any Chrome-extension, mini-program or IM-platform developer agreement
  imposes obligations on a renamed fork.

This audit was produced by an automated scan plus manual review. It is a
good-faith engineering record, not legal advice.

---

## 6. Pre-release checklist

**Legal — must be green**

- [ ] Resolve blocking item 1.1 (`cedar-go`, GPL-2.0 in every binary)
- [ ] Resolve blocking item 1.2 (`EbookLib`, AGPL-3.0)
- [ ] Resolve blocking item 1.3 (`chardet`, LGPL-2.1)
- [x] `TencentSans.ttf` removed (unused `@font-face`; deleted together with `fonts.css`)
- [ ] Re-run `tools/license_check.sh` with a full (multi-GOOS) module cache
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

- [ ] `.env`: rename every `WEKNORA_*` key to `YUHENG_*` (the server warns but
      does not fail if you forget)
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
