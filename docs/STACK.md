# Stack Guide

> **Source of truth for this project's concrete technologies, tools, and conventions.**
>
> The SDD pipeline (`plan` / `work` / `ship`) is specialized for web applications but stack-neutral
> within that: this file is where it learns what to actually run. `docs/playbooks/work.md` reads
> the [Fast Gate](#fast-gate) and [Required Tooling](#required-tooling) tables verbatim;
> `docs/playbooks/ship.md` reads [Full Gate](#full-gate) and [Release Gate](#release-gate) verbatim.
> Keep these tables accurate.
>
> **Stack status:** CONFIGURED

---

## Stack

| Layer | Technology |
|-------|-----------|
| Frontend | React + TanStack Start + Mantine UI v9+ (`@mantine/charts` for time-series/live charts); TanStack Query drives cache invalidation over the WS stream |
| Backend | Go 1.2x (`x/crypto/ssh` for SSH-based collectors) |
| Database | SQLite (`modernc.org/sqlite` or `mattn`) |
| Cache | — (no separate cache layer; batched direct writes to SQLite) |
| Infra | Current Docker image contains the Go API and adapters; web is built separately. Combined local/management-VPS distribution is deferred to M11. `infraegev2` is the first dogfood integration and owns its own target automation |
| Package managers | Go modules (backend), `pnpm` (frontend) |
| CI | GitHub Actions (`.github/workflows/ci.yml`) on pull requests and pushes to `main` |

---

## Prerequisites

```bash
# Examples — replace with the actual versions this project requires
# go version
# node --version
# pnpm --version
```

---

## Initial setup

```bash
# How a developer brings the stack up the first time.
# go mod tidy
# pnpm install && pnpm dev
```

---

## Fast Gate

Run by `/work` after each Backlog item or Architect Review Note, scoped to the touched area only —
not the full suite. Fill every row that applies; mark `n/a` for rows that don't (e.g. no frontend
→ frontend rows are `n/a`). Reported as `SKIPPED — n/a in STACK.md` otherwise.

| Check | Command | Preconditions / notes |
|-------|---------|-----------------------|
| Lint | `test -z "$(find cmd internal adapters -type f -name '*.go' -print0 \| xargs -0 gofmt -l)" && go vet ./...` (backend) / `pnpm --dir web lint` (frontend) | Native Go checks avoid a machine-local linter dependency |
| Type-check (affected) | `pnpm --dir web typecheck` | frontend only; Go is statically checked by `go vet`/tests |
| Targeted / affected unit tests | `go test ./... -short` (backend) / `pnpm --dir web vitest related` (frontend) | |
| LSP diagnostics | `no — not yet available, add gopls for Go` | informational — enforced via Required Tooling, not a shell command |
| API type regen (`openapi-typescript` or equivalent) | `node scripts/api-contracts.mjs` (regen) / `node scripts/api-contracts.mjs --check` (drift check) | run when the API surface changed — see § API contract generation |

---

## Full Gate

Run once by `/ship`, before merging a change's feature branch into `main`. Do not run this per
task — it's expensive by design; that's why it's separated from the Fast Gate.

| Check | Command | Preconditions / notes |
|-------|---------|-----------------------|
| Infrastructure / bootstrap | `n/a` | Source-quality gate; self-contained runtime distribution is deferred to M11 |
| Migrations | `n/a` | SQLite schema managed inline; revisit if a migration tool is adopted |
| Backend formatting / static analysis | `test -z "$(find cmd internal adapters -type f -name '*.go' -print0 \| xargs -0 gofmt -l)" && go vet ./...` | Must produce no unformatted files or vet findings |
| Go module integrity | `go mod verify` | |
| Backend test suite | `go test ./...` | |
| Frontend install | `pnpm --dir web install --frozen-lockfile` | pnpm 10.33.0, Node.js 24 |
| API contract drift | `go install github.com/swaggo/swag/cmd/swag@v1.16.6 && PATH="$PWD/web/node_modules/.bin:$PATH" node scripts/api-contracts.mjs --check` | Requires the preceding frontend install |
| Frontend lint | `pnpm --dir web lint` | |
| Frontend build / route generation | `pnpm --dir web build` | Must precede type-check in a clean checkout because `routeTree.gen.ts` is generated and ignored |
| Frontend type-check | `pnpm --dir web typecheck` | Requires the preceding build-generated route tree |
| Frontend unit tests | `pnpm --dir web test` | |
| E2E lint / determinism | `n/a` | no e2e suite yet |
| E2E (Playwright) | `n/a` | no e2e suite yet |
| Smoke | `go build ./...` | Source/build smoke only until M11 defines the packaged runtime |
| SAST (e.g. Semgrep) | `n/a` | not set up yet |
| Secrets scan (e.g. Gitleaks) | `n/a` | not set up yet |
| Dependency audit (e.g. Trivy / `npm audit` / `pip-audit`) | `n/a` | not set up yet |
| Accessibility audit (e.g. axe / Lighthouse CI) | `n/a` | not set up yet |
| Performance budget (e.g. Lighthouse CI, Core Web Vitals) | `n/a` | not set up yet |

If the project ships a helper script, declare it:

```bash
# ./scripts/ship.sh [XX]
```

---

## Release Gate

Run only by `/ship --release`, after the Full Gate has passed and the change is merged locally —
before pushing to `origin/main`.

| Check | Command | Preconditions / notes |
|-------|---------|-----------------------|
| Container image scan (e.g. Trivy) | `n/a` | not set up yet |
| Published artifact / deploy verification | `n/a` | Deferred to M11; a green CI run is not a deployed release |
| `gh` authenticated for this repo | `gh auth status` | Required before pushing |

After the Release Gate passes and `main` is pushed, the ship playbook's mandatory deployment-status
step is an exact-commit CI check (not a deployment claim): locate the `ci.yml` run whose
`headSha` equals `git rev-parse main`, then run `gh run watch <database-id> --exit-status` and
confirm the completed run still reports that SHA via `gh run view`.

---

## Required Tooling

Mandatory tools/skills per domain — `/work` enforces these before checking an item off; a mandated
tool that isn't available must be reported as skipped with a reason, never silently omitted.

| Domain | Required tool/skill | When | Available in this project |
|--------|----------------------|------|-----------------------------|
| Frontend UI change | Playwright MCP / chrome-devtools MCP (screenshot + console check) | after implementing, before checking off | `yes` (Playwright MCP) |
| TypeScript / Python change | LSP diagnostics | after implementing, before checking off | `no — recommend adding gopls for Go; TS LSP status unconfirmed` |
| New/changed API surface | `openapi-typescript` (or equivalent) regen + frontend re-typecheck | after backend contract change | `yes` — `swag` annotations + `node scripts/api-contracts.mjs` (see § API contract generation) |
| Architecture-level decision | architecture skill | during planning | `no` |
| Frontend design decision | `frontend-design` skill | during `/plan` §5.3 and design Backlog items | `no` |
| Backend/API design decision | `backend-design` skill | during `/plan` §4 and backend-architecture Backlog items | `no` |

Mark a row `no` (not available) rather than leaving it blank — an unmarked row is otherwise
ambiguous between "not asked" and "not needed."

---

## Testing

### Backend

```bash
go test ./...
```

### Frontend (if applicable)

```bash
pnpm tsc --noEmit
pnpm vitest run
```

---

## Project structure

```
.
├── docs/
│   ├── SPEC.md              # vision/contract anchor
│   ├── STACK.md              # this file
│   ├── FRONTEND_CONVENTIONS.md # binding frontend coding conventions
│   ├── KNOWN_GOTCHAS.md      # recurring pitfalls
│   ├── CHANGE_TEMPLATE.md    # template for new changes
│   ├── changes/              # active units of work
│   │   └── archive/          # completed units of work
│   └── playbooks/            # plan.md / work.md / ship.md / workflow-init.md
├── .claude/skills/            # Claude Code skill wrappers (plan, work, ship)
├── .agents/skills/             # generic-agent skill wrappers (plan, work, ship)
├── plugins/sdd-workflow/       # Codex plugin (skills, commands, MCP, hooks)
├── cmd/, internal/, adapters/  # Go backend — see § Backend Architecture below
├── web/                        # React frontend — see § Frontend Architecture below
└── AGENTS.md / CLAUDE.md       # AI agent rules
```

---

## Backend Architecture (Go, DDD-like)

> Style reference: ported from `/home/niquetamerewsl/projects/infraegev2`'s FastAPI backend —
> specifically its vertical-slice module boundaries, manual composition root
> (`create_app()` + settings singleton, no DI container/framework), and exception-hierarchy-to-HTTP
> mapping. That reference project does **not** itself have a domain/repository layer, so the
> `domain` → `application` → `infrastructure` → `interfaces` split below is added on top to make
> this genuinely DDD rather than "FastAPI module folders in Go."

Each bounded context is a vertical slice with its own internal DDD layers, plus one shared
`internal/platform` package for cross-cutting concerns:

```
cmd/server/main.go                         # composition root — manual wiring, no DI framework

internal/platform/config/config.go          # typed settings from env
internal/platform/db/sqlite.go              # connection + migration runner
internal/platform/db/migrations/0001_init.sql
                                             # sources, metrics, checks, events, alerts,
                                             # alert_rules (tables only — no Go module reads/writes
                                             # alerts/alert_rules until M5), metrics_rollup (reserved)
internal/platform/secrets/secrets.go        # secrets.enc.json read/write — shared kernel used by
                                             # sources.secret_ref and auth's password hash
internal/platform/httpserver/server.go      # HTTP server bootstrap + router mount
internal/platform/httpserver/middleware.go  # request-id, error logging
internal/platform/apierror/apierror.go      # domain-error -> HTTP status mapping, one place

internal/contract/schema.go                 # go:embed contract.schema.json; validates NDJSON
                                             # lines; schema_version / additive-only rule lives here

internal/sources/
  domain/source.go            # Source entity + Repository interface + domain errors
  application/service.go      # CreateSource/UpdateSource/Enable/Disable/Delete/List use-cases
  infrastructure/sqlite_repo.go
  interfaces/http/handlers.go # /api/sources (GET/POST/PATCH/DELETE)

internal/telemetry/
  domain/{metric,check,event}.go   # entities + Repository interfaces
  application/service.go           # Ingest use-case (implements adapterengine's TelemetryIngestor
                                    # port) + Query use-cases
  infrastructure/sqlite_repo.go
  interfaces/http/handlers.go      # /api/metrics, /api/checks, /api/events

internal/adapterengine/       # named to avoid colliding with the top-level adapters/ dir
                               # (external subprocess adapter binaries)
  domain/manifest.go          # Manifest, Mode(pull|stream) + validation
  application/ports.go        # TelemetryIngestor port (interface) — adapterengine depends on this
                               # abstraction, not concretely on internal/telemetry; main.go wires
                               # telemetry.application.Service as the implementation
  application/runner.go       # pull-mode: subprocess exec, NDJSON parse, contract validation,
                               # auto-disable after 10 consecutive invalid lines
  application/scheduler.go    # per-source interval invocation of pull adapters
  application/supervisor.go   # stream-mode: keep-alive, heartbeat watch, backoff restart
  infrastructure/subprocess.go # OS process spawn / stdio pipe plumbing
  interfaces/http/handlers.go  # /api/adapters (installed adapters + manifests)

internal/auth/
  domain/session.go
  application/service.go        # Login use-case, session validation
  interfaces/http/handlers.go   # /api/auth/login
  interfaces/http/middleware.go # session-required middleware, mounted in platform/httpserver

adapters/stub/main.go, manifest.json   # external stub adapter binary, different namespace from
                                        # internal/adapterengine on purpose
```

**Cross-module dependencies use ports, not direct imports.** When one bounded context needs
another (e.g. `adapterengine` needs to write telemetry), the consuming module defines the
interface it needs (a "port") in its own `application` package, and the composition root
(`cmd/server/main.go`) wires the concrete implementation in. Modules never import another module's
`application`/`infrastructure` package directly — this is the boundary discipline the reference
project's FastAPI modules did *not* enforce (its router handlers cross-import other modules'
`service.py` directly); don't repeat that here.

**Error handling**: each module's `domain` package defines typed/sentinel errors (e.g.
`sources.ErrNotFound`); `internal/platform/apierror` maps them to HTTP status in one place —
mirrors the reference's `AppException` base + subclass-per-error pattern, in idiomatic Go form.

**Testing**: Go-idiomatic co-located `_test.go` files next to the code they test (not a flat
top-level `tests/` dir). Tests exercising only `domain`/`application` logic (no real I/O) run
under Fast Gate's `go test ./... -short`; anything touching `infrastructure`/`interfaces` (real
SQLite, real HTTP) is gated behind `testing.Short()` so it only runs in Full Gate's plain
`go test ./...`.

**Deferred, not yet built**: `internal/alerts` (full domain/CRUD/router — targeted for M5, once
the Alert router itself is designed); `internal/platform/wshub` and the `/api/stream` WS handler
(targeted for M4, when the live dashboard needs it).

### API contract generation

`swaggo/swag` doc-comment annotations on `internal/*/interfaces/http/handlers.go` functions
generate `contracts/openapi.json`; `openapi-typescript` then generates
`web/src/shared/api/schema.ts` from that. A root `scripts/api-contracts.mjs` (plain `node`, `.mjs`
extension so no root `package.json` is needed) orchestrates both steps:

```bash
node scripts/api-contracts.mjs          # regenerate contracts/openapi.json + web schema.ts
node scripts/api-contracts.mjs --check  # generate into a temp dir, diff against the committed
                                         # files, fail on drift — same technique as the reference
                                         # project's scripts/api-contracts.mjs, with its Python
                                         # `app.openapi()` export step replaced by `go generate
                                         # ./...` (running swag)
```

**This is a different contract from `contract.schema.json`** (above — the NDJSON adapter data
contract for Metric/Check/Event/Alert). Same word, two unrelated schemas, deliberately named and
located differently (`contract.schema.json` at repo root vs. `contracts/openapi.json` in a
subdirectory) so they're never confused: one describes what adapters emit over stdio, the other
describes the HTTP/WS surface the frontend consumes.

---

## Frontend Architecture (FSD-like)

> Binding coding conventions (naming, destructuring, effects, types, Mantine policy components,
> testing) live in [`docs/FRONTEND_CONVENTIONS.md`](./FRONTEND_CONVENTIONS.md) — this section is
> the layer/tooling contract those conventions assume, not a restatement of them.

> Style reference: ported from `/home/niquetamerewsl/projects/infraegev2`'s `apps/web` — same
> stack (React + TanStack Start + TanStack Query + Mantine), genuinely FSD, so this translates
> almost directly.

```
web/
├── package.json, vite.config.ts, tsconfig.json
├── eslint.config.js         # layer-boundary rules wired from the start (no-restricted-imports
│                             # groups — same technique as the reference, no extra lint plugin)
└── src/
    ├── routes/               # TanStack Start file-based routing — thin, composition-only
    ├── pages/
    ├── widgets/
    ├── features/
    ├── entities/
    └── shared/
        ├── api/              # client.ts (openapi-fetch instance), errors.ts (ApiError,
        │                     # normalizeApiFailure), schema.ts (generated — see § API contract
        │                     # generation above), index.ts (public API)
        ├── components/       # Mantine-wrapping policy components (Typography, Image,
        │                     # ExternalLink, PageContainer), route-state (RoutePending/
        │                     # RouteError/RouteNotFound), empty-state, navigation-progress,
        │                     # client-error-monitor — root component colocated with
        │                     # *.types.ts + optional *.module.css; no forced ui/ sub-segment;
        │                     # complex slices get a components/ subfolder
        ├── config/           # client-env.ts (one place reading import.meta.env),
        │                     # mantine-theme.ts (owns theme/defaults — see docs/SPEC.md §5.3
        │                     # for the actual palette/typography tokens it's populated with)
        ├── lib/               # safe-ls.ts, safe-json.ts, query-client.ts (retry: false —
        │                     # matches SPEC §5.2's WS-push cache model), client-errors/
        │                     # (capture/fingerprint/dedupe; submit sink stubbed, no backend
        │                     # endpoint yet); the /api/stream WebSocket wrapper lives here
        │                     # once M4 needs it
        └── styles/
```

Conventions, ported near-verbatim from the reference:
- **Layers populate incrementally.** Folders exist now, empty; code is added only when a real
  page/feature needs it — don't force ceremony code into `entities`/`features` just to fill them.
- **Every populated slice exposes one `index.ts` public-API barrel.** Deep imports and "upward"
  imports (e.g. `shared` importing from `widgets`) are forbidden via `eslint.config.js`
  `no-restricted-imports` groups.
- **No global state library.** TanStack Query is the only "state," matching `docs/SPEC.md` §5.2's
  WS-store design — add a store only when a real cross-route owner appears.
- `shared/api`'s pattern resolves the "API type regen" row in the Fast Gate table below — see
  § API contract generation above for the Go-side half of that pipeline.

### Frontend tooling

- **`web/package.json` scripts**: `dev`, `build`, `start`, `test` (vitest), `test:e2e`
  (playwright), `typecheck` (`tsc --noEmit`), `lint`/`lint:fix` — ESLint, then
  `web/scripts/verify-app-architecture.mjs` (a self-test that lints known-good/known-bad code
  snippets against the config below and asserts the right rule fires, guarding the policy against
  silent regressions), chained in that order.
- **ESLint technique** (`web/eslint.config.js`), ported from the reference:
  - Per-FSD-layer `no-restricted-imports`: each layer may only import from itself and layers
    below it (§ tree above), and cross-slice imports must go through a slice's `index.ts` — no
    deep imports.
  - Policy-component bans: `no-restricted-imports`/`no-restricted-syntax` forbidding raw
    `<a>`/`<img>` and direct Mantine `Anchor`/`Container`/`Image`/`Text`/`Title` outside `shared`
    (see `docs/FRONTEND_CONVENTIONS.md` § 8).
  - `no-restricted-globals` banning `window`/`document`/`navigator`/`localStorage`/
    `sessionStorage`/`fetch`/`process` everywhere, with a narrow per-file allow-list for the one
    owning adapter of each (e.g. only `shared/api/client.ts` may call `fetch`, only
    `shared/lib/safe-ls.ts` may touch `localStorage`/`window`).
- **Prettier**: default config (no overrides) + `.prettierignore` excluding generated files
  (`src/routeTree.gen.ts`, `shared/api/schema.ts`, `contracts/openapi.json`) and `*.md`.
- **TypeScript**: `strict: true`, `noUnusedLocals`/`noUnusedParameters`, `~/*` → `src/*` path
  alias (mirrored in `vite.config.ts` and `vitest.config.ts`).
- **Vite**: `@tanstack/react-start/plugin/vite` + `@vitejs/plugin-react`, `resolve:
  { tsconfigPaths: true }`.
- **Testing config**: `web/tests/render.tsx` is the themed Vitest render helper (wraps units in
  the production Mantine theme). `web/e2e/fixtures.ts` + `web/e2e/pages/*.page.ts` hold the
  Playwright fixture/Page-Object scaffolding per `docs/FRONTEND_CONVENTIONS.md` § 9 — no specs
  exist until real pages land at M4.

---

## Common operations

```bash
# Start the stack
# [command]

# Stop everything
# [command]

# Add a new migration / schema change
# [command]

# Format / lint
# [command]
```
