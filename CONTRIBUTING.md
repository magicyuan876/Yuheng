# Contributing to Yuheng

Thanks for considering a contribution. Yuheng is maintained by one person, so
small, focused changes that come with tests are the ones that get merged fastest.

## Before you start

- **Bugs and questions**: open an issue with the template that fits. For a
  security problem, do *not* open an issue; follow [`SECURITY.md`](./SECURITY.md).
- **Larger changes** (a new feature, a new dependency, anything that changes a
  public API or the database schema): open an issue first and agree on the
  approach. A pull request nobody asked for and that pulls in a new framework is
  hard to accept.
- Contributions are licensed under the [MIT License](./LICENSE) of the project.
  By opening a pull request you confirm that you wrote the change or have the
  right to submit it under that license.

## Working on the code

The repository layout and conventions are described in [`CLAUDE.md`](./CLAUDE.md)
(written for AI collaborators, and just as useful for people). In short:

- **Backend** (`internal/`, Go): `gofumpt` formatting, `golangci-lint` v2 with
  the repository configuration (line length 120). PostgreSQL is the only
  database, in production and in tests; tests that need one call
  `pgtest.New(t)`, which starts a real ParadeDB container, so **Docker must be
  running for `go test ./...`**. Tests must pass in any time zone.
- **Frontend** (`frontend/`, Vue 3 + TypeScript): new code uses Tailwind and
  shadcn-vue; the older TDesign/Less screens are being retired one file at a
  time, and a component is either entirely new-stack or entirely old-stack.
- **Comments** explain *why*, in full English sentences.
- **Locales**: user-visible strings go into all four of `en-US`, `zh-CN`,
  `ko-KR` and `ru-RU`.

## Checks that must pass

```bash
# backend (repository root)
go build ./... && go vet ./... && golangci-lint run --new-from-rev=origin/main ./... && go test ./...

# frontend
npm --prefix frontend run format:check
npm --prefix frontend run lint
npm --prefix frontend test
npm --prefix frontend run type-check
npm --prefix frontend run check-i18n
NODE_OPTIONS=--max-old-space-size=4096 npm --prefix frontend run build
```

CI runs the same checks, plus a license and notice check (`tools/license_check.sh`)
and a secret scan. A new dependency must come with its entry in
`THIRD_PARTY_NOTICES.md` and a license the project can ship under MIT.

## Pull requests

- One logical change per pull request; keep formatting-only changes in a
  separate one.
- Title in the Conventional Commits style: `type(scope): imperative summary`.
- The description says *why* the change is needed. Bugs you find on the way are
  named in the description, not silently fixed.
- Add or update tests. A fix without a test that fails before it is unlikely to
  be merged.
