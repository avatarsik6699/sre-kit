# CHANGE 04 — Minimal UI

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `04` |
| Slug | `minimal-ui` |
| Title | Minimal UI |
| Status | `archived` |
| Branch | `feature/04-minimal-ui` |

---

## Goal

Deliver M4 from `docs/SPEC.md` §9: the minimal live UI — Sources, Dashboard, and Source detail
pages — so an architect can add a source through the UI and see its data with no manual API calls,
per SPEC §1.2 success metric. This change adds the `/api/stream` WebSocket endpoint (Metric/Check/
Event pub/sub by `source_id`, per SPEC §4/§5.2 — `alert` frames are part of the wire shape but no
`alert` producer exists yet, see M5) on top of the existing `internal/sources` and
`internal/telemetry` backends from changes 01–03, then builds the FSD-layer frontend on the
skeleton scaffolded in change 01: WS stream store, TanStack Query cache layered over it, status
tile, live chart, schema-driven add-source drawer, and the Dashboard / Sources / Source detail
pages per SPEC §5.1–§5.2, using the design system baseline already recorded in SPEC §5.3. Login
page/flow against the existing `/api/auth/login` endpoint is included since every other route
requires a session. Notifications page and alert-rule UI are out of scope — no alert-rule or
notification-channel backend exists yet (M5).

---

## Design References

<!-- none provided -->

---

## Backlog

### Backend
- [x] `B1` `internal/platform/wshub` — in-process pub/sub hub (per `docs/STACK.md` § Backend Architecture "Deferred, not yet built"): register/unregister connections, subscribe/unsubscribe by `source_id`, `Publish(sourceID, frame)` fan-out — _Depends on:_ —
- [x] `B2` `internal/telemetry/application` — wire `IngestMetric`/`IngestCheck`/`IngestEvent` to call B1's hub (via a small port interface so `telemetry/application` doesn't import `net/http`/websocket libs) after each successful write — _Depends on:_ B1
- [x] `B3` `internal/telemetry/interfaces/http` — `GET /api/stream` WebSocket handler: session-gated upgrade, subscribe/unsubscribe-by-`source_id` client messages, pushes new Metric/Check/Event frames from B1's hub; on connect the client does its own REST snapshot fetch (per SPEC §5.2), this endpoint only streams deltas — _Depends on:_ B1, B2
- [x] `B4` `swag` doc-comment annotations for B3's handler so `contracts/openapi.json`/`schema.ts` cover the WS upgrade route's auth requirement — _Depends on:_ B3

### Frontend
- [x] `F1` Login page (`/login`) — admin-password form against `/api/auth/login`, redirect to `/` on success, route guard on all other routes redirecting to `/login` when unauthenticated — _Depends on:_ —
- [x] `F2` WS stream store (`web/src/shared/lib`) — single `/api/stream` connection, pub/sub by currently-visible `source_id`s, reconnect-then-resnapshot behavior per SPEC §5.2 — _Depends on:_ B3, F1
- [x] `F3` TanStack Query layer over F2 for `/api/metrics`, `/api/checks`, `/api/events` (historical/paginated reads), cache entries invalidated by WS push, no polling — _Depends on:_ F2
- [x] `F4` Status tile component (`web/src/entities` or `web/src/widgets` per FSD boundary) — name, status-pulse dot, sparkline, check summary, per SPEC §5.2/§5.3 signature pulse motif — _Depends on:_ F3
- [x] `F5` Live chart component (`@mantine/charts`) — time-series with live-append plus 24h/7d historical window toggle, mono axis labels per SPEC §5.3 typography — _Depends on:_ F3
- [x] `F6` Add-source drawer — schema-driven form generated from `GET /api/adapters` manifest `config_schema`, with a test-connection step before `POST /api/sources` — _Depends on:_ F1
- [x] `F7` Sources page (`/sources`) — list of sources with live status, enable/disable/remove actions (`PATCH`/`DELETE /api/sources/:id`), opens F6 drawer to add — _Depends on:_ F4, F6
- [x] `F8` Dashboard page (`/`) — dense responsive grid of F4 status tiles across all sources, live-updating via F2/F3 — _Depends on:_ F4
- [x] `F9` Source detail page (`/sources/:id`) — F5 live chart plus a dense live event feed for the one source, using the pulse motif per event line — _Depends on:_ F5
- [x] `F10` Left icon+label collapsible rail nav per SPEC §5.3 layout, wired to Dashboard/Sources/Login routes — _Depends on:_ F8, F7

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
internal/platform/wshub/hub.go
internal/telemetry/application/service.go
internal/telemetry/application/ports.go
internal/telemetry/interfaces/http/handlers.go
internal/telemetry/interfaces/http/stream.go
cmd/server/main.go (wire wshub into telemetry service + stream handler)
contracts/openapi.json (regenerated)
web/src/routes/**
web/src/pages/**
web/src/widgets/**
web/src/entities/**
web/src/features/**
web/src/shared/api/schema.ts (regenerated)
web/src/shared/lib/** (WS store)
web/src/shared/config/** (rail nav, route guard wiring if needed)

# Added during F1-F10 verification — see Architect Review Notes for why each was necessary:
web/src/shared/api/client.ts (SSR baseUrl + session-cookie forwarding)
internal/platform/httpserver/middleware.go (Hijack passthrough so WS upgrades survive logging middleware)
internal/platform/httpserver/middleware_test.go
internal/sources/application/service.go (MarkSeen — see "Do NOT touch" deviation below)
internal/sources/application/service_test.go
internal/sources/application/export_test.go
~~~

### Do NOT touch
- `internal/sources/**`, `internal/auth/**` domain/application logic (consume existing use-cases
  only; do not change their contracts)
- `internal/adapterengine/**` (no adapter-engine changes in this UI change)
- Alert/alert-rule/notification-channel code — none exists yet, out of scope until M5
- `docs/changes/archive/**`

---

## Contracts

See `docs/SPEC.md` §3–§5 (Data Model, API/Backend Contract, Frontend/Client Contract) and the
Files list above. Do not hand-copy schema, endpoint, or type details into this file.

---

## Gate Checks

> Fast Gate runs per task in `/work`; Full Gate and (with `--release`) Release Gate run once in
> `/ship`. Both are defined in [docs/STACK.md](../../STACK.md) — this section only records
> change-specific overrides.

None.

---

## Architect Review Notes

- [x] `R1` Deliberate deviation from the "Do NOT touch `internal/sources/**` application logic"
  boundary above: added `Service.MarkSeen(ctx, id, status)` to
  `internal/sources/application/service.go`. Found while doing F1-F10's live-verification pass
  (Playwright against a real running backend, per `docs/STACK.md`'s UI-testing requirement):
  `sources.last_status`/`last_seen_at` were set to `"unreachable"`/`nil` at `Create` and **nothing
  in the existing codebase ever updated them** (confirmed by grep — no write site outside
  `Create`), so F4's status-pulse dot (the change's signature motif, SPEC §5.2/§5.3) stayed
  permanently grey/unreachable on every source regardless of real check results, even though
  telemetry ingestion itself worked (check-summary text like "2 ok" rendered correctly — this was
  purely the rollup field being dead). User confirmed the deviation explicitly (chose "fix it now"
  over "leave it, note it in the change doc" when asked) since the alternative — leaving the
  headline live-status indicator permanently wrong — defeats this change's own goal ("see its data
  with no manual API calls"). `MarkSeen` deliberately bypasses `OnChange` (doesn't call
  `notifyChange`) so it can't re-trigger the adapter engine's schedule-reconcile hook on every
  single telemetry ingest. Wired from `internal/telemetry/application.Service` via a new
  `SourceStatusUpdater` port (mirrors the existing `Publisher` port pattern — ports, not direct
  imports, per `docs/STACK.md`) — see `internal/telemetry/application/service.go`'s
  `sourceStatusForCheck` for the check-status → source-status mapping. `cmd/server/main.go` was
  reordered so `sourcesService` exists before `telemetryService` is constructed. Covered by new
  tests in `internal/sources/application/service_test.go` and
  `internal/telemetry/application/service_test.go`.
- [x] `R2` `internal/platform/httpserver/middleware.go`'s `statusRecorder` (used by `withLogging`,
  part of `httpserver.Chain`, applied to every route including `GET /api/stream`) embedded
  `http.ResponseWriter` without forwarding `http.Hijacker`. `coder/websocket`'s `Accept` needs to
  hijack the connection to upgrade it, so every single WebSocket handshake failed with
  `501 Not Implemented` — confirmed via a direct `curl` upgrade request against the running
  backend, bypassing the frontend/dev-proxy entirely, so this wasn't a dev-proxy artifact. This
  broke all of F2/F3/F4/F5/F8/F9's live-push behavior (WS `readyState` never left `CONNECTING`,
  browser console showed the 501 on a reconnect loop). Fixed by adding a `Hijack` passthrough
  method to `statusRecorder`. Verified: a regression test
  (`internal/platform/httpserver/middleware_test.go`) fails without the fix and passes with it;
  also re-verified live in a browser (Playwright) — a stub source's live chart populated with new
  points on the Source detail page with no manual reload.
- [x] `R3` `web/src/shared/api/client.ts` used a bare relative `baseUrl`, correct client-side
  (single-origin app, SPEC §8) but broken during SSR: `_authenticated.tsx`'s `beforeLoad` runs
  server-side too (TanStack Start), and Node's `fetch` has no page origin to resolve a relative URL
  against, so every SSR-rendered route 500'd (`Failed to parse URL from /api/sources`). Separately,
  even once given an absolute SSR `baseUrl`, the SSR fetch didn't carry the browser's session
  cookie (Node has no cookie jar), so an authenticated user got bounced back to `/login` on every
  full page reload. Both fixed in the same file: `createIsomorphicFn` (TanStack Start's
  execution-model primitive) supplies an absolute `baseUrl` server-side only (dead-code-eliminated
  from the client bundle), and an `openapi-fetch` `onRequest` middleware forwards the incoming
  request's `Cookie` header via `getRequestHeader` (`@tanstack/react-start/server`) during SSR.
  Verified live: full-page reload while authenticated now stays on `/` instead of redirecting to
  `/login`.

---

## Implementation Notes

- Backend dev-run verification requires `SRE_KIT_SECRETS_KEY` (required, no default —
  `internal/platform/config/config.go`) and adapter binaries built at
  `adapters/<name>/<name>` (`go build -o adapters/<name>/<name> ./adapters/<name>` per adapter) —
  the repo doesn't ship prebuilt adapter binaries or a build step for them yet. Neither is specific
  to this change; noting it here since it cost real time to work out during F1-F10's live
  verification pass and isn't written down anywhere else.
- `pnpm start` (`node .output/server/index.mjs`) does not match this TanStack Start version's
  actual build output (`vite build` produces `dist/server/server.js`, not `.output/`) — the
  `package.json` `start` script is stale. Not fixed here (out of this change's Files list and not
  needed for `pnpm dev`, which is what Fast Gate / this change's own verification used), but worth
  a follow-up before this app is ever deployed via `pnpm start`.

---

## Commit Message

```
feat(change-04): minimal UI — Dashboard, Sources, Source detail via live WS stream
```
