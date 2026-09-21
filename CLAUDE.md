# Yuheng — working notes for AI collaborators

Yuheng (玉衡) is an LLM-powered enterprise knowledge platform: a Go backend
(`internal/`, Gin, `/api/v1`), a Vue 3 frontend (`frontend/`), a Python
document parser (`docreader/`, gRPC) and a Node collaboration service
(`collab/`, Yjs). It is an independent fork of Tencent's WeKnora, run as a
Docker Compose stack. Solo-maintained, in active development.

## How work lands

- Commit straight to `main`. No feature branches, no PRs — one developer.
- Before committing: read the diff and split it into coherent commits. Subject
  in the imperative, `type(scope): …`; body explains *why*, in prose. Bugs
  found on the way get named in the message.
- Every commit must pass the frontend gates (below) and `go test ./...`.
- Whole-tree reformats are listed in `.git-blame-ignore-revs`; never mix a
  reformat with a code change.

## Frontend (`frontend/`)

Vue 3.5 + TypeScript + Vite 7 + Pinia + vue-router + vue-i18n. **Two styling
stacks coexist while the old one is retired, screen by screen.**

| | New stack — use for all new code | Legacy — do not extend |
|---|---|---|
| Styling | Tailwind v4 utility classes | Less in `<style scoped lang="less">` |
| Components | shadcn-vue, copied into `src/components/ui/` (Reka UI underneath) | TDesign `<t-*>` |
| Icons | `@lucide/vue` (`XIcon` naming) | `<t-icon name="…">` |

Rules:

1. **A component is all-new or all-old.** Never mix `<t-*>` with shadcn, or
   Less with utilities, inside one `.vue` file. Migrating a screen means the
   whole file; its child components stay as they are (the boundary is the
   component).
2. `src/components/ui/*` is **our source**. Read it, edit it, add
   components with `npx shadcn-vue@latest add <name>` (run with proxy
   variables unset if the registry fetch fails; the CLI ignores
   `HTTPS_PROXY` handling that the rest of the toolchain has).
3. `src/views/docs/**` is the pilot module and is held strictly: no `any`
   (ESLint enforces it there and only there).
4. Toasts still go through `MessagePlugin` from `tdesign-vue-next`; that is a
   JS API, not a template component, and is replaced separately.

How the two stacks coexist is decided in **`src/assets/tailwind.css`** and the
`tdesignInLayer` plugin in `vite.config.ts`. Read those two files before
touching global styles. In short: cascade layers ordered
`theme, base, tdesign, components, utilities`; no Tailwind preflight (the
Markdown views need browser defaults); element resets are scoped to
`[data-slot]` — put `data-slot="…"` on a bare element to opt it in; every
semantic token (`bg-primary`, `text-muted-foreground`, `text-placeholder`,
`text-warning`, `rounded-md`…) is bridged to a TDesign `--td-*` variable, so
both stacks share one palette and one dark mode (`theme-mode="dark"` on
`<html>`; use the `dark:` variant).

### Commands (run in `frontend/`)

```
npm run dev            # Vite, proxies /api to localhost:8080
npm test               # Vitest, once      (npm run test:watch to watch)
npm run lint           # ESLint, zero warnings allowed
npm run format         # Prettier --write  (format:check in CI)
npm run type-check     # vue-tsc
npm run build          # needs NODE_OPTIONS=--max-old-space-size=4096 on a small machine
npm run check-i18n     # locale key parity
```

CI (`.github/workflows/frontend.yml`) runs format → lint → test → type-check
→ build. All five must be green locally before a commit.

### Tests

- Vitest + happy-dom; assertions use `node:assert/strict`. Component tests
  should mount with `@vue/test-utils`.
- Thirteen inherited tests read a component's *source as text* and
  regex-match it (`// @vitest-environment node` at the top). They are the
  weakest tests in the suite — a formatting change can fail them. Do not
  write new ones; replace them with mounted tests when you touch that area.
- Two guards worth knowing: `localeKeyAudit.test.ts` (all four locales carry
  the same keys) and `commands.test.ts` (no two slash-menu entries may read
  the same in any language). Adding a menu item means adding its label to
  `en-US`, `zh-CN`, `ko-KR`, `ru-RU`.

### Conventions

- Prettier defaults, `printWidth: 120` (matches the Go `lll` setting).
  Double quotes, semicolons. Don't argue with the formatter.
- Comments explain *why*, in full sentences; the codebase's comment density
  is high and should stay so. Write them in English.
- Unused variables and imports are errors (imports auto-fix with `--fix`).
  Unused function *parameters* are not checked.
- `@ts-ignore` is banned; `@ts-expect-error` needs a description.
- A template that uses a component it never imported is a **type error**
  (`vueCompilerOptions.checkUnknownComponents`). Vue would otherwise render
  the tag as nothing and log a warning nobody reads — that bug shipped once.
  TDesign's globally installed `<t-*>` components are made known to the
  checker by `src/types/tdesign-global.d.ts`, deliberately as `any`; the
  header of that file says when and how to replace it with TDesign's real
  types.

## Backend (`internal/`, Go 1.26)

gofmt + gofumpt, golangci-lint v2 (`.golangci.yml`, line length 120).
`make test`, `make lint`. Config is environment-driven; every knob is
documented in `.env.example` — keep it in sync with `docker-compose.yml`.

## Running it

```
docker compose --profile docs up -d      # app, frontend, collab, drawio, docreader, postgres, redis
```

Ports (this machine's `.env`): frontend `8086`, API `18081`, collab `1234`,
draw.io `8087`. The frontend image needs `frontend/dist` built first
(`scripts/build_frontend_dist.sh`). Compose hard-codes the internal service
names into `NO_PROXY` and *appends* the shell's value — a shell
`NO_PROXY=localhost,127.0.0.1` used to replace the whole list and cut
containers off from each other.

## Gotchas on this machine (Windows, corporate proxy)

- In a compound shell command the leading `cd` can be dropped by the tool
  layer. Use `git -C <repo>` and `npm --prefix <dir>` rather than relying on
  the working directory. An `npm install` that runs at the repository root
  creates a stray `package.json` there — check `git status` after installs.
- Docker Desktop's registry mirror in `daemon.json` is stale (401). Pull base
  images from `<registry-mirror>/<image>` and retag.
- `frontend/Dockerfile` pins nginx by digest; the local copy carries that
  digest under the mirror's name, so a local image build needs the pin
  removed temporarily (and restored — it must not be committed unpinned).
- Node's `fetch` does not use `HTTPS_PROXY`; npm does. If a CLI that fetches
  from the internet fails with a network error, retry with the proxy
  variables unset — direct egress works from this host.
