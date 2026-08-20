# CHANGE 01 — Core skeleton

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `01` |
| Slug | `core-skeleton` |
| Title | Core skeleton |
| Status | `archived` |
| Branch | `feature/01-core-skeleton` |

---

## Goal

Deliver M1 from `docs/SPEC.md` §9: the Go core skeleton — versioned data contract, SQLite schema,
the adapter runner (pull mode + NDJSON validation), and admin-password session auth — enough that a
stub adapter's test data is visible through `/api/metrics` and every API endpoint is inaccessible
without a valid session. Backend code follows the modular DDD-like layout in `docs/STACK.md`
§ Backend Architecture, and HTTP handlers carry `swag` annotations so `contracts/openapi.json` and
the frontend's generated `schema.ts` can be produced (§ API contract generation). This change also
scaffolds the FSD-layer frontend structure and its reusable, domain-neutral shared infrastructure
(Mantine policy wrappers, router-level error/loading/not-found handling, API client, storage
helpers, client-error capture) per `docs/STACK.md` § Frontend Architecture and
`docs/FRONTEND_CONVENTIONS.md`, so it exists before real pages/features start at M4 — no
page/feature/entity content is implemented here.

---

## Design References

<!-- none provided -->

---

## Backlog

### Data
- [x] `D1` Write `contract.schema.json` — JSON Schema for `metric`/`check`/`event`/`alert` per SPEC §4, `schema_version: "1.0"`, additive-only versioning rule documented in the file — _Depends on:_ —
- [x] `D2` SQLite schema migration (`internal/platform/db/migrations/0001_init.sql`): `sources`, `metrics`, `checks`, `events`, `alerts`, `alert_rules` (tables only — no Go code reads/writes `alerts`/`alert_rules` in this change, see Do NOT touch), `metrics_rollup` (reserved, unused) per SPEC §3 — _Depends on:_ —

### Backend
- [x] `B1` Go module + platform skeleton: `cmd/server/main.go` composition-root stub, `internal/platform/config`, `internal/platform/db` (connection + migration runner), `internal/platform/httpserver` (server bootstrap, mount point), `internal/platform/apierror` (domain-error → HTTP mapping) — establishes the module layout every later task builds inside — _Depends on:_ —
- [x] `B2` `internal/platform/secrets` — `secrets.enc.json` read/write, symmetric encryption, key from env var per SPEC §3 — _Depends on:_ B1
- [x] `B3` `internal/contract` — embeds `contract.schema.json` (`go:embed`), validates NDJSON lines against it — _Depends on:_ B1, D1
- [x] `B4` `internal/sources` — `domain` (Source entity, Repository interface, domain errors), `infrastructure` (SQLite repo), `application` (Create/Update/Enable/Disable/Delete/List use-cases), `interfaces/http` (`/api/sources` GET/POST/PATCH/DELETE) — _Depends on:_ D2, B2
- [x] `B5` `internal/telemetry` — `domain` (Metric/Check/Event entities, Repository interfaces), `infrastructure` (SQLite repo), `application` (Ingest use-case implementing the `TelemetryIngestor` port consumed by `adapterengine`, plus Query use-cases) — _Depends on:_ D2, B3
- [x] `B6` `internal/adapterengine` domain + pull-mode: `domain/manifest.go`, `application/ports.go` (`TelemetryIngestor` port), `application/runner.go` (subprocess exec, NDJSON parse, contract validation, auto-disable after 10 consecutive invalid lines), `infrastructure/subprocess.go` — _Depends on:_ B3, B5
- [x] `B7` `internal/adapterengine` scheduler + supervisor: `application/scheduler.go` (per-source interval invocation of pull adapters), `application/supervisor.go` (stream-mode keep-alive, heartbeat watch, backoff restart) — _Depends on:_ B6
- [x] `B8` `internal/adapterengine/interfaces/http` — `/api/adapters` (installed adapters + manifests) — _Depends on:_ B6
- [x] `B9` `internal/auth` — `domain` (Session, password-hash logic), `application` (Login use-case, session validation), `interfaces/http` (`/api/auth/login` handler, session-required middleware mounted in `platform/httpserver`) — _Depends on:_ B2
- [x] `B10` `internal/telemetry/interfaces/http` — `/api/metrics`, `/api/checks`, `/api/events`, session-gated via `internal/auth` middleware — _Depends on:_ B5, B9
- [x] `B11` Stub test adapter (`adapters/stub`) emitting fixed NDJSON `metric` data, to exercise the full pull-mode pipeline end-to-end — _Depends on:_ B6
- [x] `B12` Add `swag` doc-comment annotations to the HTTP handler functions in `B4`/`B8`/`B9`/`B10` (`internal/sources`, `internal/adapterengine`, `internal/auth`, `internal/telemetry` — all under `interfaces/http/handlers.go`), so `swag`/`go generate` can produce `contracts/openapi.json` per `docs/STACK.md` § API contract generation — _Depends on:_ B4, B8, B9, B10

### Infra
- [x] `I1` `Dockerfile` — single-container build of the Go binary per `docs/STACK.md` deploy target — _Depends on:_ B1
- [x] `I2` `.env`/config loading, including the secrets-encryption-key variable — _Depends on:_ B2
- [x] `I3` `web/` full tooling skeleton: `package.json` (react, react-dom, `@tanstack/react-start`, `@tanstack/react-router`, `@tanstack/react-query`, `@tanstack/react-router-ssr-query`, `@mantine/core`, `@mantine/hooks`, `@mantine/nprogress`, `@mantine/charts`, `openapi-fetch`; dev: typescript, vite, `@vitejs/plugin-react`, eslint + `typescript-eslint` + `eslint-plugin-react` + `eslint-plugin-react-hooks` + `eslint-config-prettier` + `eslint-plugin-playwright`, prettier, vitest + jsdom + `@testing-library/react`, `@playwright/test`, `openapi-typescript` — versions pinned to current-latest-stable at implementation time), `tsconfig.json` (`strict`, `~/*` → `src/*`), `vite.config.ts` (`tanstackStart` + `@vitejs/plugin-react`), `vitest.config.ts`, `playwright.config.ts`, `eslint.config.js` (base rules now; FSD-boundary/policy-component/no-restricted-globals groups filled in as `I7`/`I9`/`I13` land), `prettier.config.mjs` + `.prettierignore`, `.editorconfig` — per `docs/STACK.md` § Frontend tooling — _Depends on:_ —
- [x] `I4` Empty FSD layer folders under `web/src/`: `routes/`, `pages/`, `widgets/`, `features/`, `entities/`, `shared/{api,components,config,lib,styles}` — no logic, structure only — _Depends on:_ I3
- [x] `I5` `web/src/shared/config/{client-env,mantine-theme}.ts` — typed env access point; Mantine theme populated with the palette/typography from `docs/SPEC.md` §5.3 (dark tokens, status colors, Instrument Sans/JetBrains Mono) — _Depends on:_ I4
- [x] `I6` `web/src/shared/lib/{safe-ls,safe-json,query-client}.ts` — SSR-safe localStorage wrapper, persisted-JSON helper, `retry: false` QueryClient factory (matches SPEC §5.2's WS-push cache model) — _Depends on:_ I4
- [x] `I7` `web/src/shared/components/{typography,image,external-link,page-container}/` — Mantine policy-wrapper components per `docs/FRONTEND_CONVENTIONS.md` § 8 (folder = `component.tsx` + `component.types.ts` + `index.ts`) — _Depends on:_ I5
- [x] `I8` `web/src/shared/components/{route-state,empty-state,navigation-progress}/` — `RoutePending`/`RouteError`/`RouteNotFound`, `EmptyState`, nav progress bar — _Depends on:_ I7
- [x] `I9` `web/src/shared/lib/client-errors/` + `web/src/shared/components/client-error-monitor/` — global error capture/fingerprint/dedupe pipeline; submit sink is a stubbed interface (logs locally, no network call) — no `/api/client-errors` backend endpoint in this change — _Depends on:_ I6
- [x] `I10` `web/src/router.tsx` + `web/src/routes/__root.tsx` — wire `createRouter`'s `defaultPendingComponent`/`defaultErrorComponent`/`defaultNotFoundComponent` to `I8`'s components, mount `ClientErrorMonitor` + nav progress in the root document — _Depends on:_ I8, I9
- [x] `I11` `web/tests/render.tsx` (themed Vitest render helper) + `web/e2e/fixtures.ts` + `web/e2e/pages/.gitkeep` — Playwright fixture/POM scaffolding per `docs/FRONTEND_CONVENTIONS.md` § 9, no specs yet — _Depends on:_ I3
- [x] `I12` Go side of API contract generation: `swag` wiring (`go generate` directive) producing `contracts/openapi.json`; `scripts/api-contracts.mjs` orchestrator with `--check` drift mode per `docs/STACK.md` § API contract generation — _Depends on:_ B12
- [x] `I13` `web/src/shared/api/{client,errors,index}.ts` — `openapi-fetch` client wrapper, `ApiError`/`normalizeApiFailure` — _Depends on:_ I12

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
contract.schema.json
go.mod
go.sum
cmd/server/main.go
internal/platform/config/config.go
internal/platform/db/sqlite.go
internal/platform/db/migrations/0001_init.sql
internal/platform/secrets/secrets.go
internal/platform/httpserver/server.go
internal/platform/httpserver/middleware.go
internal/platform/apierror/apierror.go
internal/contract/schema.go
internal/sources/domain/source.go
internal/sources/application/service.go
internal/sources/infrastructure/sqlite_repo.go
internal/sources/interfaces/http/handlers.go
internal/telemetry/domain/metric.go
internal/telemetry/domain/check.go
internal/telemetry/domain/event.go
internal/telemetry/application/service.go
internal/telemetry/infrastructure/sqlite_repo.go
internal/telemetry/interfaces/http/handlers.go
internal/adapterengine/domain/manifest.go
internal/adapterengine/application/ports.go
internal/adapterengine/application/runner.go
internal/adapterengine/application/scheduler.go
internal/adapterengine/application/supervisor.go
internal/adapterengine/infrastructure/subprocess.go
internal/adapterengine/interfaces/http/handlers.go
internal/auth/domain/session.go
internal/auth/application/service.go
internal/auth/interfaces/http/handlers.go
internal/auth/interfaces/http/middleware.go
adapters/stub/main.go
adapters/stub/manifest.json
Dockerfile
.env.example
contracts/openapi.json
scripts/api-contracts.mjs
web/package.json
web/vite.config.ts
web/vitest.config.ts
web/playwright.config.ts
web/tsconfig.json
web/eslint.config.js
web/prettier.config.mjs
web/.prettierignore
web/.editorconfig
web/src/routes/.gitkeep
web/src/pages/.gitkeep
web/src/widgets/.gitkeep
web/src/features/.gitkeep
web/src/entities/.gitkeep
web/src/router.tsx
web/src/routes/__root.tsx
web/src/shared/config/client-env.ts
web/src/shared/config/mantine-theme.ts
web/src/shared/lib/safe-ls.ts
web/src/shared/lib/safe-json.ts
web/src/shared/lib/query-client.ts
web/src/shared/lib/client-errors/reporter.ts
web/src/shared/lib/client-errors/browser-adapter.ts
web/src/shared/lib/client-errors/index.ts
web/src/shared/components/typography/typography.tsx
web/src/shared/components/typography/typography.types.ts
web/src/shared/components/typography/index.ts
web/src/shared/components/image/image.tsx
web/src/shared/components/image/image.types.ts
web/src/shared/components/image/index.ts
web/src/shared/components/external-link/external-link.tsx
web/src/shared/components/external-link/external-link.types.ts
web/src/shared/components/external-link/index.ts
web/src/shared/components/page-container/page-container.tsx
web/src/shared/components/page-container/page-container.types.ts
web/src/shared/components/page-container/index.ts
web/src/shared/components/route-state/route-state.tsx
web/src/shared/components/route-state/index.ts
web/src/shared/components/empty-state/empty-state.tsx
web/src/shared/components/empty-state/index.ts
web/src/shared/components/navigation-progress/navigation-progress.tsx
web/src/shared/components/navigation-progress/index.ts
web/src/shared/components/client-error-monitor/client-error-monitor.tsx
web/src/shared/components/client-error-monitor/index.ts
web/src/shared/api/client.ts
web/src/shared/api/errors.ts
web/src/shared/api/index.ts
web/src/shared/api/schema.ts
web/src/shared/styles/.gitkeep
web/tests/render.tsx
web/e2e/fixtures.ts
web/e2e/pages/.gitkeep
~~~

### Do NOT touch
- `DRAFT_SPEC.md` — historical brief, superseded by `docs/SPEC.md` as source of truth
- `internal/alerts` — do not create this module. Alert CRUD, rule evaluation, and the firing/
  resolved lifecycle are deferred to M5, once the Alert router itself is designed. The
  `alerts`/`alert_rules` tables in `D2`'s migration exist for forward-compat only; no Go code in
  this change reads or writes them.
- `internal/platform/wshub` and any `/api/stream` handler — deferred to M4 when the live dashboard
  needs it. Do not scaffold the WS hub in this change.
- Any `web/src/{pages,widgets,features,entities}` content, and any real HTTP call target for the
  client-error submit sink (`/api/client-errors` does not exist — see `I9`) — no page/feature
  content and no backend error-reporting endpoint in this change. Only `shared/` infra and empty
  layer folders are in scope; real pages/features start at M4.

---

## Contracts

See `docs/SPEC.md` §3–§4 (and §5–§7 where relevant) and the Files list above. Do not hand-copy the
schema, endpoints, types, or env vars into this file — the codebase and `SPEC.md` are the source
of truth; this file only tracks what to build and what's left.

---

## Gate Checks

> Fast Gate runs per task in `/work`; Full Gate and (with `--release`) Release Gate run once in
> `/ship`. Both are defined in [docs/STACK.md](../../STACK.md) — this section only records
> change-specific overrides.

```bash
# Optional change-specific smoke override
# curl -s http://localhost:8080/api/metrics -H "Cookie: session=<test-session>"
# expected: JSON array containing the stub adapter's test metric data
```

---

## Architect Review Notes

- [x] `R1` `cmd/server/main.go` never wires `internal/adapterengine`'s Scheduler/Runner to real
      sources: creating a source via `POST /api/sources` doesn't start pulling it, so
      `GET /api/metrics` stays empty. Fails this change's own documented smoke test (SPEC §9/M1:
      "a stub adapter's test data is visible through `/api/metrics`"). Wire the composition root
      to (1) construct the Scheduler with the telemetry service as `TelemetryIngestor` and the
      sources service's `Disable` as `SourceDisabler`, (2) resolve each source's adapter command
      from its installed manifest (`internal/adapterengine/application.ListInstalled`), (3) call
      `Scheduler.Schedule` for every enabled source on startup, and (4) call `Schedule`/`Cancel`
      from the sources HTTP handlers (or a port passed into them) on create/enable/disable/delete
      so the scheduler stays in sync with source state.

  **Fixed:** added `sourcesapp.Service.OnChange` (a `ChangeHook` func-type port, mirroring
  `adapterengine`'s `SourceDisabler` — `sources` still doesn't import `adapterengine` directly),
  fired after Create/Update/Delete. `cmd/server/main.go` registers a hook that resolves the
  source's adapter via `adapterengine/application.ListInstalled`, matches on manifest name +
  pull mode, and calls `Scheduler.Schedule`/`Cancel`; also reconciles all existing enabled sources
  on startup. Stream-mode sources log a "not wired yet" message instead of silently doing nothing
  (no stream adapter exists yet to wire against). One bug found and fixed while re-verifying the
  smoke test: the hook initially passed the *request's* `context.Context` into `Schedule`, so the
  scheduled goroutine's context was cancelled the moment the HTTP handler returned, killing the
  job before it ever ran. Fixed by giving the scheduler its own long-lived
  `context.Background()`, decoupled from any request. Re-verified live: creating a source now
  makes its metrics appear in `GET /api/metrics` within ~1-2s (the stub adapter's first pull), and
  disabling a source stops further scheduling with no errors logged.

---

## Implementation Notes

- `go:embed` cannot reach outside its own package directory (no `..` in patterns, and it refuses
  to follow a symlink pointing outward — "cannot embed irregular file"). The real
  `contract.schema.json` therefore lives at `internal/contract/contract.schema.json`; the
  repo-root `contract.schema.json` (per the Files list / D1) is a symlink into it, so the file
  embeds cleanly while still existing at the root path the change file names.
- Go toolchain (1.26.5) was not present in the dev environment; installed user-local to
  `~/.local/go` (symlinked into `~/.local/bin`, already on `PATH`) since `sudo apt install` wasn't
  available non-interactively. No system-wide change made.
- `web/`'s `typescript` and `eslint` are pinned to `5.9.3`/`9.39.5` rather than the literal latest
  npm releases (`7.0.2`/`10.8.1`) — `typescript-eslint@8.67.0` and `eslint-plugin-react@7.37.5`
  don't yet declare peer support for those majors, so "current-latest-stable" was read as latest
  *within what the toolchain's peer ranges actually support*, not latest in isolation.
- `RouteNotFound` (I8/I10) has no "go home" link: this change scaffolds zero page routes (Do NOT
  touch — no `pages`/`widgets`/`features`/`entities` content), so there's no valid `Link to`
  target yet. A real link home is a one-line addition once M4 adds an index route.
- Manually verified `web/src/router.tsx` + `routes/__root.tsx` in a browser (Playwright MCP,
  `pnpm dev`): dark theme, Instrument Sans, and the not-found page all render correctly with no
  console errors, after adding `mantineHtmlProps` to `<html>` (Mantine's documented fix for an SSR
  hydration mismatch on the `data-mantine-color-scheme` attribute — required, not optional, once a
  server-rendered `<html>` carries a color scheme). One remaining dev-only console warning
  (`virtual:tanstack-start-dev-client-entry` script hydration mismatch) is Vite's own HMR dev
  client script racing SSR in dev mode; it doesn't reproduce in `pnpm build`'s output, which is
  what actually ships.
- Two fixes to I12's pipeline, found while building I13 against real generated output:
  (1) `swag`'s `--dir` defaults to the invoking process's cwd, non-recursive into parent dirs — the
  `go:generate` directive's original `-g main.go` (cwd = `cmd/server`) silently produced zero
  paths since every annotated handler lives under `internal/`, outside that cwd. Fixed by scanning
  from repo root (`--dir ../../ -g cmd/server/main.go` in the directive; `-g cmd/server/main.go`
  from repo root in `scripts/api-contracts.mjs`). (2) `swag` emits Swagger 2.0, which
  `openapi-typescript` 7.x rejects outright ("Unsupported Swagger version: 2.x"); added
  `swagger2openapi` as a conversion step between the two in `scripts/api-contracts.mjs`. Re-verified
  `node scripts/api-contracts.mjs` and `--check` both produce the full 7-path
  `contracts/openapi.json` with no drift.

---

## Commit Message

```
feat(change-01): core skeleton — DDD-like Go modules, FSD-like frontend scaffold + shared infra
```
